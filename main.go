package main

import (
	"context"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	"github.com/charmbracelet/wish/activeterm"
	"github.com/charmbracelet/wish/bubbletea"
	"github.com/charmbracelet/wish/logging"
	"github.com/google/uuid"
	gossh "golang.org/x/crypto/ssh"
)

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

	sshPort := os.Getenv("SSH_PORT")
	httpPort := os.Getenv("HTTP_PORT")
	if sshPort == "" {
		sshPort = "2222"
	}
	if httpPort == "" {
		httpPort = "8080"
	}
	hostKey := os.Getenv("HOST_KEY")
	if hostKey == "" {
		log.Fatalf("error, you must provide the HOST_KEY env var")
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
		log.Printf("listening for ssh requests")
		if err = s.ListenAndServe(); err != nil && !errors.Is(err, ssh.ErrServerClosed) {
			log.Fatalf("error, starting http server. Error: %v", err)
		}
	}()

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "https://www.terminaltype.com", http.StatusFound)
	})

	go func() {
		defer cancel()
		log.Printf("listening for http requests")
		err := http.ListenAndServe(":"+httpPort, nil)
		if err != nil {
			log.Fatalf("error, when serving http. Error: %v", err)
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
	context     context.Context
	renderer    *lipgloss.Renderer
	fingerprint string
}

func NewModel(
	renderer *lipgloss.Renderer,
	fingerprint string,
) tea.Model {
	ctx := context.Background()
	return model{
		context:     ctx,
		renderer:    renderer,
		fingerprint: fingerprint,
	}
}

func (m model) Init() tea.Cmd {
	return nil
}
