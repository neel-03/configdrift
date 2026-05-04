package target

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/neel-03/configdrift/internal/config"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// SSHAdapter implements the Adapter interface for SSH targets.
type SSHAdapter struct {
	cfg    config.TargetConfig
	client *ssh.Client
	mu     sync.Mutex // guards client — multiple goroutines may call Fetch concurrently
}

// NewSSHAdapter creates a new SSH adapter for the given target config.
// defaults port to 22 if not specified.
func NewSSHAdapter(cfg config.TargetConfig) *SSHAdapter {
	if cfg.Port == 0 {
		cfg.Port = 22
	}
	return &SSHAdapter{cfg: cfg}
}

// Name returns the target name.
func (a *SSHAdapter) Name() string {
	return a.cfg.Name
}

// Fetch connects to the remote host and reads the config file.
// uses SFTP to avoid shell injection.
// if the connection is stale (e.g. server rebooted), it resets and retries once.
func (a *SSHAdapter) Fetch(ctx context.Context) ([]byte, error) {
	if err := a.ensureConnected(ctx); err != nil {
		return nil, fmt.Errorf("ssh connection failed for %s: %w", a.cfg.Host, err)
	}

	data, err := a.readFileViaSFTP()
	if err != nil {
		// connection might have gone stale, reset it and try one more time
		// before giving up.
		// this handles server reboots and idle timeouts cleanly.
		a.resetClient()

		if connErr := a.ensureConnected(ctx); connErr != nil {
			return nil, fmt.Errorf("ssh reconnect failed for %s: %w", a.cfg.Host, connErr)
		}

		data, err = a.readFileViaSFTP()
		if err != nil {
			return nil, err
		}
	}

	return data, nil
}

// readFileViaSFTP opens an SFTP subsystem over the existing SSH connection
// and reads the target file. No shell is involved, so it is safer and handles paths
// with spaces or special characters correctly.
func (a *SSHAdapter) readFileViaSFTP() ([]byte, error) {
	sftpClient, err := sftp.NewClient(a.client)
	if err != nil {
		return nil, fmt.Errorf("failed to open sftp subsystem on %s: %w", a.cfg.Host, err)
	}
	defer func() { _ = sftpClient.Close() }()

	f, err := sftpClient.Open(a.cfg.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to open remote file %s: %w", a.cfg.Path, err)
	}
	defer func() { _ = f.Close() }()

	data, err := io.ReadAll(f)
	if err != nil {
		return nil, fmt.Errorf("failed to read remote file %s: %w", a.cfg.Path, err)
	}

	return data, nil
}

// ensureConnected dials the remote host and upgrades to SSH if not already connected.
// locked with a mutex so concurrent Fetch calls don't race to create two connections.
func (a *SSHAdapter) ensureConnected(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.client != nil {
		return nil
	}

	key, err := os.ReadFile(a.cfg.Key)
	if err != nil {
		return fmt.Errorf("unable to read private key: %w", err)
	}

	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		return fmt.Errorf("unable to parse private key: %w", err)
	}

	hostKeyCallback, err := a.getHostKeyCallback()
	if err != nil {
		return fmt.Errorf("failed to setup host key verification: %w", err)
	}

	sshConfig := &ssh.ClientConfig{
		User: a.cfg.User,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: hostKeyCallback,
		Timeout:         5 * time.Second,
	}

	if a.cfg.Timeout != "" {
		if t, err := time.ParseDuration(a.cfg.Timeout); err == nil {
			sshConfig.Timeout = t
		}
	}

	addr := fmt.Sprintf("%s:%d", a.cfg.Host, a.cfg.Port)

	// dial with context so the caller can cancel (e.g. on SIGTERM or interval timeout)
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("unable to connect to %s: %w", addr, err)
	}

	// upgrade the raw TCP connection to SSH — this is where host key verification happens
	sshConn, chans, reqs, err := ssh.NewClientConn(conn, addr, sshConfig)
	if err != nil {
		// close the raw TCP conn, ssh handshake failed so nothing else will clean it up
		_ = conn.Close()
		return fmt.Errorf("unable to upgrade to ssh on %s: %w", addr, err)
	}

	a.client = ssh.NewClient(sshConn, chans, reqs)
	return nil
}

// resetClient closes the current client and nils it out so ensureConnected
// will create a fresh one on the next call. safe to call even if client is nil.
func (a *SSHAdapter) resetClient() {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.client != nil {
		_ = a.client.Close() // best-effort, ignore error
		a.client = nil
	}
}

// getHostKeyCallback returns a host key verifier in this order:
//  1. the path specified in config (explicit - most reliable)
//  2. ~/.ssh/known_hosts (user's default - convenient for local setups)
//  3. hard error - we never fall back to accepting unknown hosts
//
// skipping verification would defeat the whole point of the tool: if an attacker
// can MITM the SSH connection, they can serve a fake config and hide real drift.
func (a *SSHAdapter) getHostKeyCallback() (ssh.HostKeyCallback, error) {
	if a.cfg.KnownHosts != "" {
		cb, err := knownhosts.New(a.cfg.KnownHosts)
		if err != nil {
			return nil, fmt.Errorf("failed to read specified known_hosts %s: %w", a.cfg.KnownHosts, err)
		}
		return cb, nil
	}

	// fallback to user's known_hosts
	home, err := os.UserHomeDir()
	if err == nil {
		path := filepath.Join(home, ".ssh", "known_hosts")
		if _, statErr := os.Stat(path); statErr == nil {
			cb, err := knownhosts.New(path)
			if err != nil {
				return nil, fmt.Errorf("failed to read system known_hosts %s: %w", path, err)
			}
			return cb, nil
		}
	}

	// no known_hosts found - fail loud rather than silently trust any host key.
	// set known_hosts in source.yaml or ensure ~/.ssh/known_hosts exists.
	return nil, fmt.Errorf("no known_hosts found (checked config + ~/.ssh/known_hosts); host key verification is required - run `ssh-keyscan %s >> ~/.ssh/known_hosts` to add the host", a.cfg.Host)
}

// Close closes the underlying SSH connection.
// should be called when the adapter is no longer needed - e.g. on watcher shutdown.
func (a *SSHAdapter) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.client != nil {
		err := a.client.Close()
		a.client = nil // nil out so any stray Fetch calls reconnect cleanly
		return err
	}
	return nil
}
