package main

import (
	"context"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"database/sql"
	"embed"

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
)

var database *sql.DB
var chatClient *openai.Client
var loadingFinished = make(chan modelData, 1)
var docStyle = lipgloss.NewStyle()

//go:embed schema/*
var databaseFiles embed.FS

func main() {
	if os.Getenv("TEST_MODE") == "false" {
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

		key := os.Getenv("OPENAI_API_KEY")
		if key == "" {
			log.Fatal("error, must provide the OPENAI_API_KEY env var")
		}
		chatClient = openai.NewClient(key)

		sshPort := os.Getenv("SSH_PORT")
		if sshPort == "" {
			sshPort = "2222"
		}
		httpPort := os.Getenv("HTTP_PORT")
		if httpPort == "" {
			httpPort = "8080"
		}
		numberOfSentencesPerTypingTest := os.Getenv("NUMBER_OF_SENTENCES_PER_TYPING_TEST")
		sentencesPerTypingTestParsed := 5
		var err error
		if numberOfSentencesPerTypingTest != "" {
			sentencesPerTypingTestParsed, err = strconv.Atoi(numberOfSentencesPerTypingTest)
			if err != nil {
				log.Fatalf("error, unable to parse value provided for NUMBER_OF_SENTENCES_PER_TYPING_TEST. Provided: %v", numberOfSentencesPerTypingTest)
			}
			if sentencesPerTypingTestParsed < 1 {
				log.Fatalf("error, invalid value provided for NUMBER_OF_SENTENCES_PER_TYPING_TEST. Provided: %d", sentencesPerTypingTestParsed)
			}
		}
		hostKey := os.Getenv("HOST_KEY")
		if hostKey == "" {
			log.Fatalf("error, you must provide the HOST_KEY env var")
		}

		homeDir, err := os.UserHomeDir()
		if err != nil {
			log.Fatalf("error, could not find the home directory. Error: %v", err)
		}
		dataDirectory := fmt.Sprintf("%s/terminaltype_data/", homeDir)
		err = os.MkdirAll(dataDirectory, os.ModePerm)
		if err != nil {
			log.Fatalf("error, could not create data directory. Error: %v", err)
		}
		dbFile := fmt.Sprintf("%s%s", dataDirectory, "data")
		_, err = os.Stat(dbFile)
		if os.IsNotExist(err) {
			var file *os.File
			file, err = os.Create(dbFile)
			if err != nil {
				log.Fatalf("error, when creating db file. Error: %v", err)
			}
			file.Close()
		} else if err != nil {
			// An error other than the file not existing occurred
			log.Fatalf("error, when checking db file exists. Error: %v", err)
		}
		database, err = sql.Open("sqlite3", dbFile)
		if err != nil {
			log.Fatalf("error, when establishing connection with sqlite db. Error: %v", err)
		}
		err = processSchemaChanges(databaseFiles)
		if err != nil {
			log.Fatalf("error, when processing schema changes. Error: %v", err)
		}

		decodedKey, err := base64.StdEncoding.DecodeString(hostKey)
		if err != nil {
			log.Fatalf("error, unable to parse HOST_KEY env var. Error: %v", err)
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
			log.Fatalf("error, starting ssh server. Error: %v", err)
		}

		go func() {
			err2 := ensureEnoughGeneratedText(ctx, sentencesPerTypingTestParsed)
			if err2 != nil {
				log.Fatalf("error, when ensureEnoughGeneratedText() for main(). Error: %v", err2)
			}
		}()

		go func() {
			log.Printf("listening for ssh requests")
			if err3 := s.ListenAndServe(); err3 != nil && !errors.Is(err3, ssh.ErrServerClosed) {
				log.Fatalf("error, starting http server. Error: %v", err3)
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
				log.Fatalf("error, when serving http. Error: %v", err4)
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
	context     context.Context
	renderer    *lipgloss.Renderer
	fingerprint string
	activeView  activeView
	loading     bool
	spinner     spinner.Model
	data        modelData
	termWidth   int
	termHeight  int
}

type modelData struct {
	err error
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
