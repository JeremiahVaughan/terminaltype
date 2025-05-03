package main

import (
	"context"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/stopwatch"
	"github.com/charmbracelet/bubbles/timer"
	"github.com/muesli/termenv"


	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	"github.com/charmbracelet/wish/activeterm"
	"github.com/charmbracelet/wish/bubbletea"
	"github.com/charmbracelet/wish/logging"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/google/uuid"
	openai "github.com/sashabaranov/go-openai"
	gossh "golang.org/x/crypto/ssh"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"

    "github.com/JeremiahVaughan/terminaltype/config"
    "github.com/JeremiahVaughan/terminaltype/clients"
    "github.com/JeremiahVaughan/terminaltype/models"
    "github.com/JeremiahVaughan/terminaltype/views"
    "github.com/JeremiahVaughan/terminaltype/router"
    "github.com/JeremiahVaughan/terminaltype/controllers"
)

var ns *server.Server
var chatClient *openai.Client
var textBaseStyle lipgloss.Style
var correctStyle lipgloss.Style
var incorrectStyle lipgloss.Style
var regularStyle lipgloss.Style
var cursorStyle lipgloss.Style
var serviceName = "terminaltype"

var sentencesPerTypingTest = 3
var typingTestDesiredWidth = 60
var raceStartTimeoutInSeconds = 10
var raceTimeoutInSeconds = 180
var maxPlayersPerRace = int8(5)
var playerColors = []string{
	"#00ff00",
	"#ff5600",
	"#0000ff",
	"#ffff00",
	"#ff00ff",
}

var theClients *clients.Clients

const raceRegistrationRequestQueueId = "req_race_reg"

func main() {
    ctx, cancel := context.WithCancel(context.Background())
    signalChan := make(chan os.Signal, 1)
    signal.Notify(
        signalChan,
        os.Interrupt,
        syscall.SIGINT,
        syscall.SIGTERM,
    )

    go func() {
        <-signalChan
        cancel()
    }()

	// forces the color profile since its not getting applied sometimes
	lipgloss.SetColorProfile(termenv.TrueColor)

    config, err := config.New(ctx)
    if err != nil {
        log.Fatalf("error, when creating new config for main(). Error: %v", err)
    }

    theClients, err = clients.New(config, serviceName, healthyRefresh)
    if err != nil {
        log.Fatalf("error, when creating clients for main(). Error: %v", err)
    }

    defer theClients.Healthy.Close()

    testHealthStatus()

    models := models.New(theClients)
    views, err := views.New(config)
    if err != nil {
        err = fmt.Errorf("error, when creating views. Error: %v", err)
        HandleUnexpectedError(nil, err)
        return
    }
    controllers := controllers.New(views, models)
    router := router.New(controllers, config)


    chatClient = openai.NewClient(config.OpenAIAPIKey)

    sentencesPerTypingTest = config.NumberOfSentencesPerTypingTest
    typingTestDesiredWidth = config.TypingTestDesiredWidth
    raceStartTimeoutInSeconds = config.RaceStartTimeoutInSeconds
    maxPlayersPerRace = config.MaxPlayersPerRace
    log.Printf("make players per race: %d", maxPlayersPerRace)

    decodedKey, err := base64.StdEncoding.DecodeString(config.HostKey)
    if err != nil {
        HandleUnexpectedError(nil, fmt.Errorf("error, unable to parse HostKey. Error: %v", err))
        return
    }

    err = initNats()
    if err != nil {
        HandleUnexpectedError(nil, fmt.Errorf("error, when initNats() for main(). Error: %v", err))
        return
    }

    err = os.MkdirAll(config.Database.DataDirectory, os.ModePerm)
    if err != nil {
        HandleUnexpectedError(nil, fmt.Errorf("error, could not create data directory. Error: %v", err))
        return
    }
    dbFile := fmt.Sprintf("%s/%s", config.Database.DataDirectory, "data")
    _, err = os.Stat(dbFile)
    if os.IsNotExist(err) {
        var file *os.File
        file, err = os.Create(dbFile)
        if err != nil {
            HandleUnexpectedError(nil, fmt.Errorf("error, when creating db file. Error: %v", err))
            return
        }
        file.Close()
    } else if err != nil {
        // An error other than the file not existing occurred
        HandleUnexpectedError(nil, fmt.Errorf("error, when checking db file exists. Error: %v", err))
        return
    }

    s, err := wish.NewServer(
        wish.WithAddress(net.JoinHostPort("0.0.0.0", strconv.Itoa(config.SSHPort))),
        wish.WithHostKeyPEM(decodedKey),
        wish.WithMiddleware(
            bubbletea.Middleware(teaHandler),
            activeterm.Middleware(), // Bubble Tea apps usually require a PTY.
            logging.Middleware(),
        ),
        wish.WithPublicKeyAuth(func(_ ssh.Context, key ssh.PublicKey) bool {
            return true
        }),
        wish.WithKeyboardInteractiveAuth(
            func(ctx ssh.Context, challenger gossh.KeyboardInteractiveChallenge) bool {
                return true
            },
        ),
    )
    if err != nil {
        HandleUnexpectedError(nil, fmt.Errorf("error, starting ssh server. Error: %v", err))
        return
    }

    textBaseStyle = lipgloss.NewStyle().Width(typingTestDesiredWidth)
    correctStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#58bc54"))
    incorrectStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#ce4041"))
    regularStyle = lipgloss.NewStyle()
    cursorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#000000")).Background(lipgloss.Color("#ffffff"))

    go func() {
        err1 := handleRaceRegistration(ctx)
        if err1 != nil {
            HandleUnexpectedError(nil, fmt.Errorf("error, when handleRaceRegistration() for main(). Error: %v", err1))
            return
        }
    }()

    go func() {
        err2 := ensureEnoughGeneratedText(ctx)
        if err2 != nil {
            HandleUnexpectedError(nil, fmt.Errorf("error, when ensureEnoughGeneratedText() for main(). Error: %v", err2))
            return
        }
    }()

    go func() {
        log.Printf("listening for ssh requests")
        if err3 := s.ListenAndServe(); err3 != nil && !errors.Is(err3, ssh.ErrServerClosed) {
            HandleUnexpectedError(nil, fmt.Errorf("error, starting http server. Error: %v", err3))
            return
        }
    }()

    http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        http.Redirect(w, r, "https://www.terminaltype.com", http.StatusFound)
    })

    go func() {
        defer cancel()
        log.Printf("listening for http requests")
        err4 := router.Run(ctx, config.HTTPPort)
        if err4 != nil {
            HandleUnexpectedError(nil, fmt.Errorf("error, when serving http. Error: %v", err4))
            return
        }
    }()

    <-ctx.Done()
    s.Shutdown(ctx)
    log.Println("server shutdown properly")
}

type sshOutput struct {
	ssh.Session
	tty *os.File
}

func teaHandler(s ssh.Session) (tea.Model, []tea.ProgramOption) {
	pty, _, _ := s.Pty()
	sessionBridge := &sshOutput{
		Session: s,
		tty:     pty.Slave,
	}
	renderer := bubbletea.MakeRenderer(sessionBridge)
	fingerprint := uuid.New().String()
	publicKey := s.PublicKey()
	if publicKey != nil {
		hash := md5.Sum(publicKey.Marshal())
		fingerprint = hex.EncodeToString(hash[:])
	}
	log.Printf("ssh fingerprint from client: %s", fingerprint)
	model := NewModel(renderer, fingerprint)
	return model, []tea.ProgramOption{tea.WithAltScreen(), tea.WithMouseAllMotion()}
}

type model struct {
	ctx                  context.Context
	renderer             *lipgloss.Renderer
	fingerprint          string
	activeView           activeView
	loading              bool
	raceTicker           *stopwatch.Model
	raceStartCountDown   timer.Model
	natsConnection       *nats.Conn
	racerId              int8
	allRacerProgressChan chan *nats.Msg
	racerProgressBars    []progress.Model
	raceCtx              context.Context // used for cleaning up all resources used in the race
	raceCancel           context.CancelFunc
	spinner              spinner.Model
	data                 modelData
	raceWordsCharSlice   []string
	termWidth            int
	termHeight           int
	correctPos           int
	incorrectPos         int
	raceStartTime        int64
	wordsPerMin          int
	loadingFinished      chan modelData
}

type modelData struct {
	err              error
	raceWords        string
	wordCount        int8
	raceId           string // also the fingerprint print of user in the first race slot
	racerCount       int8
	allRacerProgress []RaceProgress
}

func NewModel(
	renderer *lipgloss.Renderer,
	fingerprint string,
) tea.Model {
	ctx := context.Background()
	m := model{
		ctx:             ctx,
		renderer:        renderer,
		fingerprint:     fingerprint,
		activeView:      activeViewWelcome,
		loadingFinished: make(chan modelData, 1),
	}
	m.resetSpinner()
	return m
}

func (m model) Init() tea.Cmd {
	return nil
}

func initNats() error {
	// Configure NATS Server options
	opts := &server.Options{
		Port: -1, // Let the server pick an available port
		// You can set other options here (e.g., authentication, clustering)
	}

	// Create a new NATS server instance
	var err error
	ns, err = server.NewServer(opts)
	if err != nil {
		return fmt.Errorf("error, when creating NATS server. Error: %v", err)
	}

	// Start the server in a separate goroutine
	go ns.Start()

	// Ensure the server has started
	if !ns.ReadyForConnections(10 * time.Second) {
		return errors.New("error, NATS Server didn't start in time")
	}

	// Retrieve the server's listen address
	addr := ns.Addr()
	var port int
	if tcpAddr, ok := addr.(*net.TCPAddr); ok {
		port = tcpAddr.Port
	} else {
		return fmt.Errorf("error, filed to get nats server port")
	}
	fmt.Printf("NATS server is running on port %d\n", port)
	return nil
}

func connectToNats() (*nats.Conn, error) {
	nc, err := nats.Connect(ns.ClientURL())
	if err != nil {
		return nil, fmt.Errorf("error, when connecting to NATS server. Error: %v", err)
	}
	return nc, nil
}

func (m *model) resetSpinner() {
	s := spinner.New()
	s.Spinner = spinner.Moon
	m.spinner = s
}

func HandleUnexpectedError(w http.ResponseWriter, err error) {
    theClients.Healthy.ReportUnexpectedError(w, err)
}

func handleRaceRegistration(ctx context.Context) error {
	conn, err := connectToNats()
	if err != nil {
		return fmt.Errorf("error, when connectToNats() for handleRaceRegistration(). Error: %v", err)
	}
	subChan := make(chan *nats.Msg)
	sub, err := conn.ChanSubscribe(raceRegistrationRequestQueueId, subChan)
	if err != nil {
		return fmt.Errorf("error, when setting up subscription for handleRaceRegistration(). Error: %v", err)
	}
	defer sub.Unsubscribe()
	defer close(subChan)
	rr := RaceRegistration{
		AllRaceProgress: make([]RaceProgress, maxPlayersPerRace),
	}
	ticker := time.Tick(time.Second)
	for {
		select {
		case natsMsg := <-subChan:
			f := string(natsMsg.Data)
			var racerAlreadyRegistered bool
			for _, theRacer := range rr.AllRaceProgress {
				if theRacer.Fingerprint == f {
					racerAlreadyRegistered = true
				}
			}
			if !racerAlreadyRegistered {
				rr.AllRaceProgress[rr.RacerCount].Fingerprint = f
				rr.AllRaceProgress[rr.RacerCount].RacerId = int8(rr.RacerCount)
				if rr.RacerCount == 0 {
					rr.RaceId = f
					rr.RaceStartTime = int64(raceStartTimeoutInSeconds) + time.Now().Unix()
				}
				rr.RacerCount++
			}
			regResponse := RegResponse{
				RaceId:        rr.RaceId,
				RaceStartTime: rr.RaceStartTime,
			}
			var resp []byte
			resp, err = json.Marshal(regResponse)
			if err != nil {
				return fmt.Errorf("error, when marshaling regResponse for handleRaceRegistration(). Error: %v", err)
			}
			err = conn.Publish(f, resp)
			if err != nil {
				return fmt.Errorf("error, when sending raceRegistrationStartTime to racer for handleRaceRegistration(). Error: %v", err)
			}
			if rr.RacerCount == maxPlayersPerRace {
				err = publishRace(conn, rr)
				if err != nil {
					return fmt.Errorf("error, when publishRace() for handleRaceRegistration() max player count was reached. Error: %v", err)
				}
				rr = RaceRegistration{
					AllRaceProgress: make([]RaceProgress, maxPlayersPerRace),
				}
			}
		case <-ticker:
			if rr.RacerCount != 0 && time.Now().Unix() >= rr.RaceStartTime {
				err = publishRace(conn, rr)
				if err != nil {
					return fmt.Errorf("error, when publishRace() for handleRaceRegistration() after race timeout exceeded. Error: %v", err)
				}
				rr = RaceRegistration{
					AllRaceProgress: make([]RaceProgress, maxPlayersPerRace),
				}
			}
		case <-ctx.Done():
			return nil
		}
	}
}

func publishRace(conn *nats.Conn, rr RaceRegistration) error {
	// start race, for now but todo try to join one first if one is available
	raceWords, wordCount, err := fetchRaceWords()
	if err != nil {
		err = fmt.Errorf("error, when fetchRaceWords() for publishRace(). Error: %v", err)
	}
	rr.RaceWords = raceWords
	rr.WordCount = wordCount
	encodedRace, err := encodeRaceRegistration(rr)
	if err != nil {
		return fmt.Errorf("error, when encodeAllRaceProgress() for handleRaceRegistration(). Error: %v", err)
	}

	for i := int8(0); i < rr.RacerCount; i++ {
		err = conn.Publish(rr.AllRaceProgress[i].Fingerprint, encodedRace)
		if err != nil {
			return fmt.Errorf("error, when publishRace() for handleRaceRegistration(). Error: %v", err)
		}
	}
	return nil
}
