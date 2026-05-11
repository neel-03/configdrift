package daemon

import (
	"os"
	"path/filepath"
	"time"
)

// RequestType defines the types of requests the daemon can handle.
type RequestType string

const (
	RequestStart WatcherRequestType = "START"
	RequestStop  WatcherRequestType = "STOP"
	RequestList  WatcherRequestType = "LIST"
	RequestPing  WatcherRequestType = "PING"
)

type WatcherRequestType string

// Request represents a JSON RPC request to the daemon.
type Request struct {
	Type   WatcherRequestType `json:"type"`
	Config string             `json:"config,omitempty"` // Path to config for START
	ID     string             `json:"id,omitempty"`     // Unique ID for STOP
}

// Response represents a JSON RPC response from the daemon.
type Response struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	ID      string      `json:"id,omitempty"` // Return the generated ID
	Data    interface{} `json:"data,omitempty"`
}

// WorkerInfo represents the state of a single watcher in the registry.
type WorkerInfo struct {
	ID         string    `json:"id"`
	ConfigPath string    `json:"config_path"`
	StartTime  time.Time `json:"start_time"`
	Status     string    `json:"status"`
}

var DefaultSocketPath string

func init() {
	home, err := os.UserHomeDir()
	if err != nil {
		DefaultSocketPath = "/tmp/configdrift.sock"
		return
	}
	dir := filepath.Join(home, ".configdrift")
	_ = os.MkdirAll(dir, 0o750)
	DefaultSocketPath = filepath.Join(dir, "configdrift.sock")
}
