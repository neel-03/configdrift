package daemon

import (
	"encoding/json"
	"fmt"
	"net"
	"time"
)

// Client communicates with the daemon server.
type Client struct {
	socketPath string
}

// NewClient creates a new daemon client.
func NewClient(socketPath string) *Client {
	if socketPath == "" {
		socketPath = DefaultSocketPath
	}
	return &Client{socketPath: socketPath}
}

// Send sends a request to the daemon and returns the response.
func (c *Client) Send(req Request) (*Response, error) {
	conn, err := net.DialTimeout("unix", c.socketPath, 2*time.Second)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to daemon: %w (is the daemon running?)", err)
	}
	defer conn.Close()

	if err := json.NewEncoder(conn).Encode(req); err != nil {
		return nil, fmt.Errorf("failed to encode request: %w", err)
	}

	var resp Response
	if err := json.NewDecoder(conn).Decode(&resp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &resp, nil
}

// IsRunning checks if the daemon is currently listening.
func (c *Client) IsRunning() bool {
	conn, err := net.DialTimeout("unix", c.socketPath, 1*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}
