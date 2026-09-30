// Package web serves the operator UI and its JSON API on 127.0.0.1.
package web

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"devupdater/internal/runner"
	"devupdater/internal/workspace"
)

type Options struct {
	Workspace string // optional workspace opened at start
	Addr      string // listen address; default 127.0.0.1:0
	ConfigDir string // recent.json location; default UserConfigDir/devupdater
	Token     string // launch token; default random
	Dial      runner.DialFunc
	Probe     runner.ProbeFunc
}

type Server struct {
	opt     Options
	token   string
	mux     *http.ServeMux
	handler http.Handler
	recent  *recentStore
	ln      net.Listener

	mu sync.Mutex
	ws string
}

func New(opt Options) (*Server, error) {
	if opt.Addr == "" {
		opt.Addr = "127.0.0.1:0"
	}
	if opt.ConfigDir == "" {
		opt.ConfigDir = os.Getenv("DEVUPDATER_CONFIG_DIR") // tests and e2e isolate state
	}
	if opt.ConfigDir == "" {
		base, err := os.UserConfigDir()
		if err != nil {
			base = os.TempDir()
		}
		opt.ConfigDir = filepath.Join(base, "devupdater")
	}
	s := &Server{opt: opt, token: opt.Token, mux: http.NewServeMux(), recent: newRecentStore(opt.ConfigDir)}
	if s.token == "" {
		b := make([]byte, 24)
		if _, err := rand.Read(b); err != nil {
			return nil, err
		}
		s.token = hex.EncodeToString(b)
	}
	s.routes()
	s.handler = s.guard(s.mux)
	if opt.Workspace != "" {
		if err := s.openWorkspace(opt.Workspace, false); err != nil {
			return nil, err
		}
	}
	return s, nil
}

func (s *Server) Handler() http.Handler { return s.handler }
func (s *Server) Token() string         { return s.token }

// routes is the single registration point; later tasks add their handlers here.
func (s *Server) routes() {
	s.registerWorkspace()
	s.registerData()
	s.mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "Bilinmeyen API adresi: "+r.URL.Path)
	})
	s.mux.Handle("/", staticHandler())
}

func (s *Server) workspace() (workspace.Workspace, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return workspace.Workspace{Dir: s.ws}, s.ws != ""
}

func (s *Server) setWorkspace(dir string) {
	s.mu.Lock()
	s.ws = dir
	s.mu.Unlock()
	s.recent.add(dir)
}

// Listen binds the address and returns the launch URL carrying the token.
func (s *Server) Listen() (string, error) {
	ln, err := net.Listen("tcp", s.opt.Addr)
	if err != nil {
		return "", err
	}
	s.ln = ln
	return fmt.Sprintf("http://%s/?token=%s", ln.Addr().String(), s.token), nil
}

// Serve serves until ctx is cancelled, then shuts down gracefully.
func (s *Server) Serve(ctx context.Context) error {
	if s.ln == nil {
		return errors.New("web: Listen must be called before Serve")
	}
	hs := &http.Server{Handler: s.handler, ReadHeaderTimeout: 10 * time.Second}
	errc := make(chan error, 1)
	go func() { errc <- hs.Serve(s.ln) }()
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		sctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		s.shutdownHooks()
		hs.Shutdown(sctx)
		return nil
	}
}

// shutdownHooks is extended in Task 6 to cancel an active run.
func (s *Server) shutdownHooks() {}
