package launcher

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"syscall"
	"time"

	"github.com/nishantdania/ark/internal/vmapi"
)

const maxBodyBytes = 64 << 10

type Config struct {
	SocketPath string
	StateDir   string
	RuntimeDir string
	SocketGID  int
	AllowedUID int
	Authorize  func(int) bool
}

func DefaultConfig() Config {
	return Config{SocketPath: "/run/ark/vm-launcher.sock", StateDir: "/var/lib/ark-vm-launcher", RuntimeDir: "/run/ark", SocketGID: -1}
}

type Server struct {
	config   Config
	runtime  Runtime
	http     *http.Server
	listener net.Listener
}

func NewServer(config Config, runtime Runtime) (*Server, error) {
	if config.SocketPath == "" || runtime == nil {
		return nil, fmt.Errorf("launcher configuration: %w", vmapi.ErrInvalid)
	}
	if config.Authorize == nil {
		allowed := config.AllowedUID
		config.Authorize = func(uid int) bool { return uid == allowed }
	}
	return &Server{config: config, runtime: runtime}, nil
}

func (s *Server) ListenAndServe() (serveErr error) {
	listener, err := listenSocket(s.config)
	if err != nil {
		return err
	}
	s.listener = listener
	defer func() {
		if err := os.Remove(s.config.SocketPath); err != nil && !errors.Is(err, os.ErrNotExist) && serveErr == nil {
			serveErr = fmt.Errorf("remove launcher socket: %w", err)
		}
	}()
	s.http = &http.Server{
		Handler:           s.handler(),
		ReadHeaderTimeout: 5 * time.Second,
		MaxHeaderBytes:    8 << 10,
		ConnContext: func(ctx context.Context, conn net.Conn) context.Context {
			uid, err := peerUID(conn)
			if err != nil {
				return ctx
			}
			return context.WithValue(ctx, peerKey{}, uid)
		},
	}
	serveErr = s.http.Serve(listener)
	if errors.Is(serveErr, http.ErrServerClosed) {
		return nil
	}
	return serveErr
}

func (s *Server) Shutdown(ctx context.Context) error {
	if s.http == nil {
		return nil
	}
	return s.http.Shutdown(ctx)
}

type peerKey struct{}

func (s *Server) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/create", s.create)
	mux.HandleFunc("POST /v1/start", s.start)
	mux.HandleFunc("POST /v1/stop", s.stop)
	mux.HandleFunc("POST /v1/delete", s.delete)
	mux.HandleFunc("POST /v1/inspect", s.inspect)
	mux.HandleFunc("POST /v1/list", s.list)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uid, ok := r.Context().Value(peerKey{}).(int)
		if !ok || !s.config.Authorize(uid) {
			writeError(w, http.StatusForbidden, "unauthorized Unix peer")
			return
		}
		mux.ServeHTTP(w, r)
	})
}

func (s *Server) create(w http.ResponseWriter, r *http.Request) {
	var request vmapi.CreateRequest
	if !decode(w, r, &request) || vmapi.ValidateCreate(request) != nil {
		writeError(w, http.StatusBadRequest, "invalid create request")
		return
	}
	if err := s.runtime.Create(r.Context(), request.Spec); err != nil {
		writeRuntimeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (s *Server) start(w http.ResponseWriter, r *http.Request) {
	var request vmapi.IDRequest
	if !decode(w, r, &request) || vmapi.ValidateID(request) != nil {
		writeError(w, http.StatusBadRequest, "invalid start request")
		return
	}
	ip, err := s.runtime.Start(r.Context(), request.ID)
	if err != nil {
		writeRuntimeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, vmapi.StartResponse{GuestIP: ip})
}
func (s *Server) inspect(w http.ResponseWriter, r *http.Request) {
	var request vmapi.IDRequest
	if !decode(w, r, &request) || vmapi.ValidateID(request) != nil {
		writeError(w, http.StatusBadRequest, "invalid inspect request")
		return
	}
	vm, err := s.runtime.Inspect(r.Context(), request.ID)
	if err != nil {
		writeRuntimeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, vmapi.InspectResponse{State: vm})
}
func (s *Server) list(w http.ResponseWriter, r *http.Request) {
	var request vmapi.VersionRequest
	if !decode(w, r, &request) || vmapi.ValidateVersion(request) != nil {
		writeError(w, http.StatusBadRequest, "invalid list request")
		return
	}
	vms, err := s.runtime.List(r.Context())
	if err != nil {
		writeRuntimeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, vmapi.ListResponse{VMs: vms})
}
func (s *Server) stop(w http.ResponseWriter, r *http.Request) { s.idOperation(w, r, s.runtime.Stop) }
func (s *Server) delete(w http.ResponseWriter, r *http.Request) {
	s.idOperation(w, r, s.runtime.Delete)
}
func (s *Server) idOperation(w http.ResponseWriter, r *http.Request, operation func(context.Context, string) error) {
	var request vmapi.IDRequest
	if !decode(w, r, &request) || vmapi.ValidateID(request) != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}
	if err := operation(r.Context(), request.ID); err != nil {
		writeRuntimeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func decode(w http.ResponseWriter, r *http.Request, value any) bool {
	defer r.Body.Close()
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return false
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return false
	}
	return true
}
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, vmapi.ErrorResponse{Error: message})
}
func writeRuntimeError(w http.ResponseWriter, err error) {
	if errors.Is(err, vmapi.ErrConflict) {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	if errors.Is(err, vmapi.ErrNotFound) {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		writeError(w, http.StatusRequestTimeout, err.Error())
		return
	}
	writeError(w, http.StatusInternalServerError, "launcher operation failed")
}

func listenSocket(config Config) (net.Listener, error) {
	if err := os.MkdirAll(config.RuntimeDir, 0750); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(config.StateDir, 0750); err != nil {
		return nil, err
	}
	if err := removeStaleSocket(config.SocketPath); err != nil {
		return nil, err
	}
	listener, err := net.Listen("unix", config.SocketPath)
	if err != nil {
		return nil, err
	}
	if err := os.Chmod(config.SocketPath, 0660); err != nil {
		_ = listener.Close()
		return nil, err
	}
	if config.SocketGID >= 0 {
		if err := os.Chown(config.SocketPath, 0, config.SocketGID); err != nil {
			_ = listener.Close()
			return nil, err
		}
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
		_ = conn.Close()
		return fmt.Errorf("launcher socket %q is active", path)
	}
	if !errors.Is(err, syscall.ECONNREFUSED) {
		return fmt.Errorf("probe launcher socket %q: %w", path, err)
	}
	return os.Remove(path)
}
