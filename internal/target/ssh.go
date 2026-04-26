package target

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/neel-03/configdrift/internal/config"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// SSHAdapter implements the Adapter interface for SSH targets.
type SSHAdapter struct {
	cfg    config.TargetConfig
	client *ssh.Client
}

// NewSSHAdapter creates a new SSH adapter.
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
func (a *SSHAdapter) Fetch(ctx context.Context) ([]byte, error) {
	if err := a.ensureConnected(ctx); err != nil {
		return nil, fmt.Errorf("ssh connection failed for %s: %w", a.cfg.Host, err)
	}

	session, err := a.client.NewSession()
	if err != nil {
		return nil, fmt.Errorf("failed to create ssh session: %w", err)
	}
	defer session.Close()

	var b bytes.Buffer
	session.Stdout = &b

	// using cat to read the file.
	// TODO: have to look for best way to read the file across platforms
	cmd := fmt.Sprintf("cat %s", a.cfg.Path)
	if err := session.Run(cmd); err != nil {
		return nil, fmt.Errorf("failed to run command %q: %w", cmd, err)
	}

	return b.Bytes(), nil
}

func (a *SSHAdapter) ensureConnected(ctx context.Context) error {
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

	// dial to the host with context cancellation
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("unable to connect: %w", err)
	}

	sshConn, chans, reqs, err := ssh.NewClientConn(conn, addr, sshConfig)
	if err != nil {
		return fmt.Errorf("unable to upgrade to ssh: %w", err)
	}

	a.client = ssh.NewClient(sshConn, chans, reqs)
	return nil
}

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
		if _, err := os.Stat(path); err == nil {
			cb, err := knownhosts.New(path)
			if err != nil {
				return nil, fmt.Errorf("failed to read system known_hosts %s: %w", path, err)
			}
			return cb, nil
		}
	}

	return nil, fmt.Errorf("no known_hosts provided and system known_hosts not found; host key verification required for production")
}

// Close closes the underlying SSH connection.
func (a *SSHAdapter) Close() error {
	if a.client != nil {
		return a.client.Close()
	}
	return nil
}
