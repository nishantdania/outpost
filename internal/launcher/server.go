package launcher

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/nishantdania/outpost/internal/api"
)

type Config struct {
	SocketPath string
	StateDir   string
	RuntimeDir string
	SocketGID  int
}

func DefaultConfig() Config {
	return Config{SocketPath: api.SocketPath, StateDir: "/srv/outpost/state", RuntimeDir: filepath.Dir(api.SocketPath), SocketGID: -1}
}

// Server exposes the launcher's application operations directly to local clients.
// Access is controlled by the root-owned socket and its parent directory, not a
// second network service or bearer tokens. Socket-group membership is privileged.
type Server struct {
	config Config
	http   *http.Server
}

func NewServer(config Config, handler http.Handler) (*Server, error) {
	if !filepath.IsAbs(config.SocketPath) || filepath.Clean(config.SocketPath) != config.SocketPath || filepath.Dir(config.SocketPath) != config.RuntimeDir || handler == nil {
		return nil, fmt.Errorf("launcher socket must be a clean absolute path inside its runtime directory, with a handler")
	}
	return &Server{config: config, http: &http.Server{
		Handler: handler, ReadHeaderTimeout: 5 * time.Second, MaxHeaderBytes: 8 << 10,
	}}, nil
}

func (s *Server) ListenAndServe() (serveErr error) {
	listener, err := listenSocket(s.config)
	if err != nil {
		return err
	}
	defer func() {
		if err := os.Remove(s.config.SocketPath); err != nil && !errors.Is(err, os.ErrNotExist) && serveErr == nil {
			serveErr = fmt.Errorf("remove launcher socket: %w", err)
		}
	}()
	serveErr = s.http.Serve(listener)
	if errors.Is(serveErr, http.ErrServerClosed) {
		return nil
	}
	return serveErr
}

func (s *Server) Shutdown(ctx context.Context) error { return s.http.Shutdown(ctx) }
func (s *Server) Close() error                       { return s.http.Close() }

func listenSocket(config Config) (net.Listener, error) {
	if err := os.MkdirAll(config.RuntimeDir, 0750); err != nil {
		return nil, err
	}
	if err := removeStaleSocket(config.SocketPath); err != nil {
		return nil, err
	}
	listener, err := net.Listen("unix", config.SocketPath)
	if err != nil {
		return nil, err
	}
	mode := os.FileMode(0600)
	if config.SocketGID >= 0 {
		if err := os.Chown(config.SocketPath, 0, config.SocketGID); err != nil {
			listener.Close()
			return nil, err
		}
		mode = 0660
	}
	if err := os.Chmod(config.SocketPath, mode); err != nil {
		listener.Close()
		return nil, err
	}
	return listener, nil
}

func removeStaleSocket(path string) error {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSocket == 0 {
		return fmt.Errorf("refusing to remove non-socket %q", path)
	}
	conn, err := net.DialTimeout("unix", path, time.Second)
	if err == nil {
		conn.Close()
		return fmt.Errorf("launcher socket %q is active", path)
	}
	if !errors.Is(err, syscall.ECONNREFUSED) {
		return fmt.Errorf("probe launcher socket %q: %w", path, err)
	}
	return os.Remove(path)
}
