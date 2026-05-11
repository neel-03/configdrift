package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"os"
	"sync"
	"time"

	"github.com/neel-03/configdrift/internal/config"
	"github.com/neel-03/configdrift/internal/utils"
	"github.com/neel-03/configdrift/internal/watcher"
)

type worker struct {
	info   WorkerInfo
	cancel context.CancelFunc
}

// Server handles background watcher management.
type Server struct {
	socketPath string
	registry   map[string]*worker
	mu         sync.RWMutex
}

// NewServer creates a new daemon server.
func NewServer(socketPath string) *Server {
	if socketPath == "" {
		socketPath = DefaultSocketPath
	}
	return &Server{
		socketPath: socketPath,
		registry:   make(map[string]*worker),
	}
}

// Start runs the daemon server.
func (s *Server) Start(ctx context.Context) error {
	// Cleanup existing socket
	_ = os.Remove(s.socketPath)

	lc := net.ListenConfig{}
	l, err := lc.Listen(ctx, "unix", s.socketPath)
	if err != nil {
		return fmt.Errorf("failed to listen on socket %s: %w", s.socketPath, err)
	}
	defer l.Close()
	defer os.Remove(s.socketPath)

	slog.Info("Daemon started", "socket", s.socketPath)

	go func() {
		<-ctx.Done()
		l.Close()
	}()

	for {
		conn, err := l.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			slog.Error("Failed to accept connection", "error", err)
			continue
		}
		go s.handleConnection(conn)
	}
}

func (s *Server) handleConnection(conn net.Conn) {
	defer conn.Close()

	var req Request
	if err := json.NewDecoder(conn).Decode(&req); err != nil {
		s.sendResponse(conn, Response{Success: false, Message: "Invalid request format"})
		return
	}

	switch req.Type {
	case RequestStart:
		s.handleStart(conn, req)
	case RequestStop:
		s.handleStop(conn, req)
	case RequestList:
		s.handleList(conn)
	case RequestPing:
		s.sendResponse(conn, Response{Success: true, Message: "PONG"})
	default:
		s.sendResponse(conn, Response{Success: false, Message: "Unknown request type"})
	}
}

func (s *Server) handleStart(conn net.Conn, req Request) {
	if req.Config == "" {
		s.sendResponse(conn, Response{Success: false, Message: "Config path required"})
		return
	}

	// Load policy
	cfg, err := config.Load(req.Config)
	if err != nil {
		s.sendResponse(conn, Response{Success: false, Message: fmt.Sprintf("Failed to load config: %v", err)})
		return
	}

	// Generate a short ID (like Docker)
	id := utils.GenerateID()

	s.mu.Lock()
	// Ensure ID uniqueness
	for _, exists := s.registry[id]; exists; _, exists = s.registry[id] {
		id = utils.GenerateID()
	}

	ctx, cancel := context.WithCancel(context.Background())

	w, err := watcher.FromPolicy(ctx, cfg, nil)
	if err != nil {
		cancel()
		s.mu.Unlock()
		s.sendResponse(conn, Response{Success: false, Message: fmt.Sprintf("Failed to create watcher: %v", err)})
		return
	}

	wrk := &worker{
		info: WorkerInfo{
			ID:         id,
			ConfigPath: req.Config,
			StartTime:  time.Now(),
			Status:     "RUNNING",
		},
		cancel: cancel,
	}
	s.registry[id] = wrk
	s.mu.Unlock()

	// Start the watcher in background
	go func() {
		if err := w.Run(ctx); err != nil && err != context.Canceled {
			slog.Error("Watcher failed", "id", id, "config", req.Config, "error", err)
			s.mu.Lock()
			if w, ok := s.registry[id]; ok {
				w.info.Status = fmt.Sprintf("FAILED: %v", err)
			}
			s.mu.Unlock()
		}
	}()

	s.sendResponse(conn, Response{
		Success: true,
		Message: fmt.Sprintf("Watcher started for %s", req.Config),
		ID:      id,
	})
}

func (s *Server) handleStop(conn net.Conn, req Request) {
	if req.ID == "" {
		s.sendResponse(conn, Response{Success: false, Message: "Watcher ID required"})
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	resolvedID, err := s.resolveID(req.ID)
	if err != nil {
		s.sendResponse(conn, Response{Success: false, Message: err.Error()})
		return
	}

	wrk := s.registry[resolvedID]
	wrk.cancel()
	delete(s.registry, resolvedID)

	s.sendResponse(conn, Response{Success: true, Message: fmt.Sprintf("Watcher %s stopped", resolvedID)})
}

// resolveID finds a full ID from a prefix.
func (s *Server) resolveID(prefix string) (string, error) {
	// 1. Direct match
	if _, exists := s.registry[prefix]; exists {
		return prefix, nil
	}

	// 2. Prefix match
	var matches []string
	for id := range s.registry {
		if len(prefix) > 0 && len(id) >= len(prefix) && id[:len(prefix)] == prefix {
			matches = append(matches, id)
		}
	}

	if len(matches) == 0 {
		return "", fmt.Errorf("watcher not found for ID/prefix %s", prefix)
	}
	if len(matches) > 1 {
		return "", fmt.Errorf("ambiguous ID prefix %s (matches: %v)", prefix, matches)
	}

	return matches[0], nil
}

func (s *Server) handleList(conn net.Conn) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	workers := make([]WorkerInfo, 0, len(s.registry))
	for _, w := range s.registry {
		workers = append(workers, w.info)
	}

	s.sendResponse(conn, Response{Success: true, Data: workers})
}

func (s *Server) sendResponse(conn net.Conn, resp Response) {
	if err := json.NewEncoder(conn).Encode(resp); err != nil {
		slog.Error("Failed to send response", "error", err)
	}
}
