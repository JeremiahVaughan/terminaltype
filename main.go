package main

import (
	"context"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/muesli/termenv"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"database/sql"
	"embed"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	"github.com/charmbracelet/wish/activeterm"
	"github.com/charmbracelet/wish/bubbletea"
	"github.com/charmbracelet/wish/logging"
	"github.com/getsentry/sentry-go"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/google/uuid"
	openai "github.com/sashabaranov/go-openai"
	gossh "golang.org/x/crypto/ssh"
)

var database *sql.DB
var chatClient *openai.Client
var loadingFinished = make(chan modelData, 1)
var sentencesPerTypingTest = 3
var typingTestDesiredWidth = 60
var textBaseStyle lipgloss.Style
var correctStyle lipgloss.Style
var incorrectStyle lipgloss.Style
var regularStyle lipgloss.Style
var cursorStyle lipgloss.Style

//go:embed schema/*
var databaseFiles embed.FS

func main() {

	// forces the color profile since its not getting applied sometimes
	lipgloss.SetColorProfile(termenv.TrueColor)

	if os.Getenv("TEST_MODE") == "false" {

		environment := os.Getenv("TF_VAR_environment")
		if environment == "" {
			log.Fatal("error, must provide the TF_VAR_environment env var")
		}

		sentryEndpoint := os.Getenv("TF_VAR_sentry_end_point")
		if sentryEndpoint == "" {
			log.Fatal("error, must provide the TF_VAR_sentry_end_point env var")
		}

		err := sentry.Init(sentry.ClientOptions{
			Dsn: sentryEndpoint,
			// Set TracesSampleRate to 1.0 to capture 100%
			// of transactions for performance monitoring.
			// We recommend adjusting this value in production,
			TracesSampleRate: 1.0,

			Environment: environment,
		})
		if err != nil {
			log.Fatalf("error, sentry.Init(). Error: %v", err)
		}
		defer sentry.Flush(2 * time.Second)

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

		key := os.Getenv("TF_VAR_openai_api_key")
		if key == "" {
			HandleUnexpectedError(nil, errors.New("error, must provide the TF_VAR_openai_api_key env var"))
			return
		}
		chatClient = openai.NewClient(key)

		sshPort := os.Getenv("TF_VAR_ssh_port")
		if sshPort == "" {
			sshPort = "2222"
		}
		httpPort := os.Getenv("TF_VAR_http_port")
		if httpPort == "" {
			httpPort = "8080"
		}
		numberOfSentencesPerTypingTestString := os.Getenv("TF_VAR_number_of_sentences_per_typing_test")
		if numberOfSentencesPerTypingTestString != "" {
			sentencesPerTypingTest, err = strconv.Atoi(numberOfSentencesPerTypingTestString)
			if err != nil {
				HandleUnexpectedError(nil, fmt.Errorf("error, unable to parse value provided for TF_VAR_number_of_sentences_per_typing_test. Provided: %v", numberOfSentencesPerTypingTestString))
				return
			}
			if sentencesPerTypingTest < 1 {
				HandleUnexpectedError(nil, fmt.Errorf("error, invalid value provided for TF_VAR_number_of_sentences_per_typing_test. Provided: %d", sentencesPerTypingTest))
				return
			}
		}
		typingTestDesiredWidthString := os.Getenv("TF_VAR_typing_test_desired_width")
		if typingTestDesiredWidthString != "" {
			typingTestDesiredWidth, err = strconv.Atoi(typingTestDesiredWidthString)
			if err != nil {
				HandleUnexpectedError(nil, fmt.Errorf("error, unable to parse value provided for TF_VAR_typing_test_desired_width. Provided: %v", typingTestDesiredWidthString))
				return
			}
			if typingTestDesiredWidth < 5 {
				HandleUnexpectedError(nil, fmt.Errorf("error, invalid value provided for TF_VAR_typing_test_desired_width. Provided: %d", typingTestDesiredWidth))
				return
			}
		}
		hostKey := os.Getenv("TF_VAR_host_key")
		if hostKey == "" {
			HandleUnexpectedError(nil, errors.New("error, you must provide the TF_VAR_host_key env var"))
			return
		}
		decodedKey, err := base64.StdEncoding.DecodeString(hostKey)
		if err != nil {
			HandleUnexpectedError(nil, fmt.Errorf("error, unable to parse TF_VAR_host_key env var. Error: %v", err))
			return
		}

		homeDir, err := os.UserHomeDir()
		if err != nil {
			HandleUnexpectedError(nil, fmt.Errorf("error, could not find the home directory. Error: %v", err))
			return
		}
		dataDirectory := fmt.Sprintf("%s/terminaltype_data/", homeDir)
		err = os.MkdirAll(dataDirectory, os.ModePerm)
		if err != nil {
			HandleUnexpectedError(nil, fmt.Errorf("error, could not create data directory. Error: %v", err))
			return
		}
		dbFile := fmt.Sprintf("%s%s", dataDirectory, "data")
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
		database, err = sql.Open("sqlite3", dbFile)
		if err != nil {
			HandleUnexpectedError(nil, fmt.Errorf("error, when establishing connection with sqlite db. Error: %v", err))
			return
		}
		err = processSchemaChanges(databaseFiles)
		if err != nil {
			HandleUnexpectedError(nil, fmt.Errorf("error, when processing schema changes. Error: %v", err))
			return
		}

		s, err := wish.NewServer(
			wish.WithAddress(net.JoinHostPort("0.0.0.0", sshPort)),
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
			err4 := http.ListenAndServe(":"+httpPort, nil)
			if err4 != nil {
				HandleUnexpectedError(nil, fmt.Errorf("error, when serving http. Error: %v", err4))
				return
			}
		}()

		<-ctx.Done()
		s.Shutdown(ctx)
		log.Println("server shutdown properly")
	}
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
	context            context.Context
	renderer           *lipgloss.Renderer
	fingerprint        string
	activeView         activeView
	loading            bool
	spinner            spinner.Model
	data               modelData
	raceWordsCharSlice []string
	termWidth          int
	termHeight         int
	correctPos         int
	incorrectPos       int
	raceStartTime      int64
	wordsPerMin        int
}

type modelData struct {
	err       error
	raceWords string
	wordCount int
}

func NewModel(
	renderer *lipgloss.Renderer,
	fingerprint string,
) tea.Model {
	ctx := context.Background()
	m := model{
		context:     ctx,
		renderer:    renderer,
		fingerprint: fingerprint,
		activeView:  activeViewWelcome,
	}
	m.resetSpinner()
	return m
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m *model) resetSpinner() {
	s := spinner.New()
	s.Spinner = spinner.Moon
	m.spinner = s
}

func HandleUnexpectedError(w http.ResponseWriter, err error) {
	sentry.CaptureException(err)
	log.Printf("ERROR: %v", err)
	if w != nil {
		w.WriteHeader(http.StatusInternalServerError)
	}
}
