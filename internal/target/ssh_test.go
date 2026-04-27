package target

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/neel-03/configdrift/internal/config"
	"github.com/pkg/sftp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
)

// generateTestKey creates a fresh 2048-bit RSA key pair each time.
// returns the PEM-encoded private key bytes.
func generateTestKey(t *testing.T) []byte {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err, "generating test RSA key should not fail")

	return pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})
}

// writeTempFile writes data to a temp file and returns its path.
// the file is automatically removed when the test ends.
func writeTempFile(t *testing.T, prefix string, data []byte) string {
	t.Helper()
	f, err := os.CreateTemp("", prefix)
	require.NoError(t, err)
	t.Cleanup(func() { os.Remove(f.Name()) })

	_, err = f.Write(data)
	require.NoError(t, err)
	require.NoError(t, f.Close())
	return f.Name()
}

// buildKnownHostsFile writes a known_hosts file for the given host/port + public key.
// this is what lets the client trust the mock server during tests.
func buildKnownHostsFile(t *testing.T, host string, port int, pub ssh.PublicKey) string {
	t.Helper()
	// known_hosts format for non-default ports: [host]:port keytype base64key
	line := fmt.Sprintf("[%s]:%d %s", host, port, string(ssh.MarshalAuthorizedKey(pub)))
	return writeTempFile(t, "known_hosts_", []byte(line))
}

// ── in-memory SFTP handler ───────────────────────────────────────────────────

// mockSFTPHandler is a read-only in-memory filesystem for the SFTP subsystem.
// it just needs to serve files by path — no writes, no dir listings beyond stat.
type mockSFTPHandler struct {
	files map[string][]byte // remote path -> file content
}

// Fileread handles open/read requests. returns a ReaderAt over the in-memory content.
func (h *mockSFTPHandler) Fileread(r *sftp.Request) (io.ReaderAt, error) {
	data, ok := h.files[r.Filepath]
	if !ok {
		return nil, os.ErrNotExist
	}
	return &memReaderAt{data: data}, nil
}

// Filewrite — not needed for configdrift; always denied.
func (h *mockSFTPHandler) Filewrite(_ *sftp.Request) (io.WriterAt, error) {
	return nil, os.ErrPermission
}

// Filecmd — not needed (no rename, remove etc.).
func (h *mockSFTPHandler) Filecmd(_ *sftp.Request) error {
	return os.ErrPermission
}

// Filelist handles stat calls — the sftp client calls Stat before Open.
func (h *mockSFTPHandler) Filelist(r *sftp.Request) (sftp.ListerAt, error) {
	data, ok := h.files[r.Filepath]
	if !ok {
		return nil, os.ErrNotExist
	}
	info := &memFileInfo{name: filepath.Base(r.Filepath), size: int64(len(data))}
	return mockListerAt([]os.FileInfo{info}), nil
}

// memReaderAt wraps a byte slice as an io.ReaderAt.
type memReaderAt struct{ data []byte }

func (m *memReaderAt) ReadAt(p []byte, off int64) (int, error) {
	if off >= int64(len(m.data)) {
		return 0, io.EOF
	}
	n := copy(p, m.data[off:])
	if n < len(p) {
		return n, io.EOF
	}
	return n, nil
}

// mockListerAt lets the SFTP server respond to Stat/Readdir requests.
type mockListerAt []os.FileInfo

func (l mockListerAt) ListAt(ls []os.FileInfo, offset int64) (int, error) {
	if offset >= int64(len(l)) {
		return 0, io.EOF
	}
	n := copy(ls, l[offset:])
	if n < len(ls) {
		return n, io.EOF
	}
	return n, nil
}

// memFileInfo is a minimal os.FileInfo for in-memory files.
type memFileInfo struct {
	name string
	size int64
}

func (f *memFileInfo) Name() string       { return f.name }
func (f *memFileInfo) Size() int64        { return f.size }
func (f *memFileInfo) Mode() os.FileMode  { return 0444 }
func (f *memFileInfo) ModTime() time.Time { return time.Time{} }
func (f *memFileInfo) IsDir() bool        { return false }
func (f *memFileInfo) Sys() interface{}   { return nil }

type mockServer struct {
	addr    string
	cleanup func()
}

// setupMockSSHServer starts an in-process SSH server that authenticates any
// public key and serves files via SFTP subsystem using the provided file map.
// the server is torn down automatically when the test ends.
func setupMockSSHServer(t *testing.T, serverKeyData []byte, files map[string][]byte) *mockServer {
	t.Helper()

	srvConfig := &ssh.ServerConfig{
		// accept any public key — we're testing the client's host key verification,
		// not the server's auth logic. don't do this outside of tests!
		PublicKeyCallback: func(_ ssh.ConnMetadata, _ ssh.PublicKey) (*ssh.Permissions, error) {
			return nil, nil
		},
	}

	signer, err := ssh.ParsePrivateKey(serverKeyData)
	require.NoError(t, err, "parsing server host key should not fail")
	srvConfig.AddHostKey(signer)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return // listener was closed — server is shutting down
			}
			go handleSSHConn(conn, srvConfig, files)
		}
	}()

	_, portStr, _ := net.SplitHostPort(listener.Addr().String())
	var port int
	fmt.Sscanf(portStr, "%d", &port)

	t.Cleanup(func() { listener.Close() })

	return &mockServer{
		addr:    listener.Addr().String(),
		cleanup: func() { listener.Close() },
	}
}

// handleSSHConn upgrades a TCP connection to SSH and routes channel requests.
func handleSSHConn(conn net.Conn, cfg *ssh.ServerConfig, files map[string][]byte) {
	_, chans, reqs, err := ssh.NewServerConn(conn, cfg)
	if err != nil {
		return // handshake failed — expected in some test cases
	}
	go ssh.DiscardRequests(reqs)

	for newChan := range chans {
		if newChan.ChannelType() != "session" {
			_ = newChan.Reject(ssh.UnknownChannelType, "only session channels supported")
			continue
		}
		ch, reqs, err := newChan.Accept()
		if err != nil {
			continue
		}
		go handleSession(ch, reqs, files)
	}
}

// handleSession services requests on an SSH session channel.
// only handles the "sftp" subsystem — which is all configdrift needs.
func handleSession(ch ssh.Channel, reqs <-chan *ssh.Request, files map[string][]byte) {
	defer ch.Close()

	for req := range reqs {
		if req.Type != "subsystem" {
			_ = req.Reply(false, nil)
			continue
		}

		// subsystem payload is a 4-byte length prefix + name
		if len(req.Payload) < 4 {
			_ = req.Reply(false, nil)
			continue
		}
		name := string(req.Payload[4:])

		if name != "sftp" {
			_ = req.Reply(false, nil)
			continue
		}

		// we have an SFTP subsystem request — spin up the request server
		_ = req.Reply(true, nil)

		handler := &mockSFTPHandler{files: files}
		srv := sftp.NewRequestServer(ch, sftp.Handlers{
			FileGet:  handler,
			FilePut:  handler,
			FileCmd:  handler,
			FileList: handler,
		})
		// blocks until the client closes the session
		_ = srv.Serve()
		return
	}
}

// addrParts splits "host:port" and returns them separately.
func addrParts(t *testing.T, addr string) (string, int) {
	t.Helper()
	host, portStr, err := net.SplitHostPort(addr)
	require.NoError(t, err)
	var port int
	fmt.Sscanf(portStr, "%d", &port)
	return host, port
}

// newAdapter is a convenience constructor for tests.
func newAdapter(host string, port int, keyFile, knownHostsFile, path string) *SSHAdapter {
	return NewSSHAdapter(config.TargetConfig{
		Name:       "test-target",
		Type:       "ssh",
		Host:       host,
		Port:       port,
		User:       "testuser",
		Key:        keyFile,
		KnownHosts: knownHostsFile,
		Path:       path,
	})
}

// ── tests ─────────────────────────────────────────────────────────────────────

func TestSSHAdapter_Name(t *testing.T) {
	// just a getter — no server needed
	a := NewSSHAdapter(config.TargetConfig{Name: "my-target"})
	assert.Equal(t, "my-target", a.Name())
}

func TestSSHAdapter_DefaultPort(t *testing.T) {
	// port should default to 22 when not provided
	a := NewSSHAdapter(config.TargetConfig{Name: "x", Port: 0})
	assert.Equal(t, 22, a.cfg.Port)
}

func TestSSHAdapter_Close_WithoutConnect(t *testing.T) {
	// closing an adapter that was never connected should be a no-op, not a panic
	a := NewSSHAdapter(config.TargetConfig{})
	assert.NoError(t, a.Close())
}

func TestSSHAdapter_Close_Idempotent(t *testing.T) {
	// calling Close twice must not panic or error — watcher shutdown may call it multiple times
	keyData := generateTestKey(t)
	keyFile := writeTempFile(t, "key_", keyData)

	signer, _ := ssh.ParsePrivateKey(keyData)
	srv := setupMockSSHServer(t, keyData, map[string][]byte{"/config.yaml": []byte("x: 1")})
	host, port := addrParts(t, srv.addr)
	knownHostsFile := buildKnownHostsFile(t, host, port, signer.PublicKey())

	a := newAdapter(host, port, keyFile, knownHostsFile, "/config.yaml")

	_, err := a.Fetch(context.Background())
	require.NoError(t, err)

	assert.NoError(t, a.Close())
	assert.NoError(t, a.Close()) // second close — should not error or panic
}

func TestSSHAdapter_Fetch_Success(t *testing.T) {
	// happy path — adapter should connect, open SFTP, and return file contents
	keyData := generateTestKey(t)
	keyFile := writeTempFile(t, "key_", keyData)

	signer, _ := ssh.ParsePrivateKey(keyData)
	files := map[string][]byte{"/etc/app/config.yaml": []byte("host: localhost\nport: 8080\n")}
	srv := setupMockSSHServer(t, keyData, files)
	host, port := addrParts(t, srv.addr)
	knownHostsFile := buildKnownHostsFile(t, host, port, signer.PublicKey())

	a := newAdapter(host, port, keyFile, knownHostsFile, "/etc/app/config.yaml")
	defer a.Close()

	data, err := a.Fetch(context.Background())
	require.NoError(t, err)
	assert.Equal(t, files["/etc/app/config.yaml"], data)
}

func TestSSHAdapter_Fetch_ReuseConnection(t *testing.T) {
	// calling Fetch twice should reuse the same SSH connection, not open a new one
	keyData := generateTestKey(t)
	keyFile := writeTempFile(t, "key_", keyData)

	signer, _ := ssh.ParsePrivateKey(keyData)
	files := map[string][]byte{"/config.yaml": []byte("x: 1")}
	srv := setupMockSSHServer(t, keyData, files)
	host, port := addrParts(t, srv.addr)
	knownHostsFile := buildKnownHostsFile(t, host, port, signer.PublicKey())

	a := newAdapter(host, port, keyFile, knownHostsFile, "/config.yaml")
	defer a.Close()

	_, err := a.Fetch(context.Background())
	require.NoError(t, err)
	firstClient := a.client // grab the client pointer

	_, err = a.Fetch(context.Background())
	require.NoError(t, err)

	// same pointer — connection was reused, not re-dialled
	assert.Same(t, firstClient, a.client, "should reuse existing SSH connection")
}

func TestSSHAdapter_Fetch_StaleConnection(t *testing.T) {
	// if the SSH connection dies between polls (server restart, idle timeout),
	// Fetch should detect the dead session and reconnect transparently
	keyData := generateTestKey(t)
	keyFile := writeTempFile(t, "key_", keyData)

	signer, _ := ssh.ParsePrivateKey(keyData)
	files := map[string][]byte{"/config.yaml": []byte("reconnected: true")}
	srv := setupMockSSHServer(t, keyData, files)
	host, port := addrParts(t, srv.addr)
	knownHostsFile := buildKnownHostsFile(t, host, port, signer.PublicKey())

	a := newAdapter(host, port, keyFile, knownHostsFile, "/config.yaml")
	defer a.Close()

	// first fetch establishes the connection
	_, err := a.Fetch(context.Background())
	require.NoError(t, err)

	// simulate stale connection — close the client but don't nil it
	// this is what happens after a server reboot or idle SSH timeout
	a.client.Close()

	// second fetch should detect the dead client, reconnect, and succeed
	data, err := a.Fetch(context.Background())
	require.NoError(t, err)
	assert.Equal(t, files["/config.yaml"], data)
}

func TestSSHAdapter_Fetch_FileNotFound(t *testing.T) {
	// SFTP should return a clear error when the remote file doesn't exist —
	// not a panic, not a generic "something went wrong"
	keyData := generateTestKey(t)
	keyFile := writeTempFile(t, "key_", keyData)

	signer, _ := ssh.ParsePrivateKey(keyData)
	// server has no files — any path will 404
	srv := setupMockSSHServer(t, keyData, map[string][]byte{})
	host, port := addrParts(t, srv.addr)
	knownHostsFile := buildKnownHostsFile(t, host, port, signer.PublicKey())

	a := newAdapter(host, port, keyFile, knownHostsFile, "/does/not/exist.yaml")
	defer a.Close()

	_, err := a.Fetch(context.Background())
	assert.Error(t, err, "fetching a non-existent remote file should return an error")
}

func TestSSHAdapter_Fetch_CancelledContext(t *testing.T) {
	// if the context is cancelled before the dial completes, Fetch should
	// return immediately with a context error — not hang
	keyData := generateTestKey(t)
	keyFile := writeTempFile(t, "key_", keyData)

	a := NewSSHAdapter(config.TargetConfig{
		Name: "unreachable",
		Host: "192.0.2.1", // TEST-NET — guaranteed unreachable, won't dial
		Port: 22,
		User: "testuser",
		Key:  keyFile,
		// no known_hosts — we won't even get that far
	})
	defer a.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err := a.Fetch(ctx)
	assert.Error(t, err, "fetch with cancelled context should return an error")
}

func TestSSHAdapter_Fetch_UnknownHostKey(t *testing.T) {
	// the server presents key A, but known_hosts has key B.
	// the adapter must reject the connection — this is the MITM protection test.
	serverKeyData := generateTestKey(t)
	serverKeyFile := writeTempFile(t, "server_key_", serverKeyData)

	differentKeyData := generateTestKey(t)
	differentSigner, _ := ssh.ParsePrivateKey(differentKeyData)

	srv := setupMockSSHServer(t, serverKeyData, map[string][]byte{})
	host, port := addrParts(t, srv.addr)

	// known_hosts has the DIFFERENT key — server will present the wrong one
	knownHostsFile := buildKnownHostsFile(t, host, port, differentSigner.PublicKey())

	a := newAdapter(host, port, serverKeyFile, knownHostsFile, "/config.yaml")
	defer a.Close()

	_, err := a.Fetch(context.Background())
	assert.Error(t, err, "should reject connection when server key doesn't match known_hosts")
}

func TestSSHAdapter_Fetch_NoKnownHostsFile(t *testing.T) {
	// when no known_hosts exists anywhere, we must fail loudly.
	// silently accepting unknown hosts would make the tool dangerous.
	keyData := generateTestKey(t)
	keyFile := writeTempFile(t, "key_", keyData)

	a := NewSSHAdapter(config.TargetConfig{
		Name:       "test",
		Host:       "127.0.0.1",
		Port:       2222,
		User:       "user",
		Key:        keyFile,
		KnownHosts: "/definitely/does/not/exist/known_hosts",
		Path:       "/config.yaml",
	})
	defer a.Close()

	_, err := a.Fetch(context.Background())
	assert.Error(t, err, "should error when specified known_hosts file does not exist")
}

func TestSSHAdapter_Fetch_MissingPrivateKey(t *testing.T) {
	// if the private key file path is wrong, error early with a clear message
	a := NewSSHAdapter(config.TargetConfig{
		Name: "test",
		Host: "127.0.0.1",
		Port: 22,
		User: "user",
		Key:  "/this/key/does/not/exist",
		Path: "/config.yaml",
	})
	defer a.Close()

	_, err := a.Fetch(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unable to read private key")
}

func TestSSHAdapter_Fetch_Concurrent(t *testing.T) {
	// run this with `go test -race` — multiple goroutines calling Fetch
	// simultaneously should not race on a.client
	keyData := generateTestKey(t)
	keyFile := writeTempFile(t, "key_", keyData)

	signer, _ := ssh.ParsePrivateKey(keyData)
	files := map[string][]byte{"/config.yaml": []byte("concurrent: true")}
	srv := setupMockSSHServer(t, keyData, files)
	host, port := addrParts(t, srv.addr)
	knownHostsFile := buildKnownHostsFile(t, host, port, signer.PublicKey())

	a := newAdapter(host, port, keyFile, knownHostsFile, "/config.yaml")
	defer a.Close()

	const goroutines = 5
	var wg sync.WaitGroup
	errs := make([]error, goroutines)

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, errs[idx] = a.Fetch(context.Background())
		}(i)
	}

	wg.Wait()

	for i, err := range errs {
		assert.NoError(t, err, "goroutine %d should not error", i)
	}
}

func TestSSHAdapter_Fetch_CustomTimeout(t *testing.T) {
	// timeout field in config should override the default 5s dial timeout
	keyData := generateTestKey(t)
	keyFile := writeTempFile(t, "key_", keyData)

	signer, _ := ssh.ParsePrivateKey(keyData)
	files := map[string][]byte{"/config.yaml": []byte("timeout: test")}
	srv := setupMockSSHServer(t, keyData, files)
	host, port := addrParts(t, srv.addr)
	knownHostsFile := buildKnownHostsFile(t, host, port, signer.PublicKey())

	a := NewSSHAdapter(config.TargetConfig{
		Name:       "test",
		Host:       host,
		Port:       port,
		User:       "testuser",
		Key:        keyFile,
		KnownHosts: knownHostsFile,
		Path:       "/config.yaml",
		Timeout:    "10s", // explicit timeout — should be accepted without error
	})
	defer a.Close()

	data, err := a.Fetch(context.Background())
	require.NoError(t, err)
	assert.Equal(t, files["/config.yaml"], data)
}

func TestSSHAdapter_Fetch_EmptyFile(t *testing.T) {
	// an empty remote file is valid — return empty bytes, not an error
	keyData := generateTestKey(t)
	keyFile := writeTempFile(t, "key_", keyData)

	signer, _ := ssh.ParsePrivateKey(keyData)
	files := map[string][]byte{"/empty.yaml": {}}
	srv := setupMockSSHServer(t, keyData, files)
	host, port := addrParts(t, srv.addr)
	knownHostsFile := buildKnownHostsFile(t, host, port, signer.PublicKey())

	a := newAdapter(host, port, keyFile, knownHostsFile, "/empty.yaml")
	defer a.Close()

	data, err := a.Fetch(context.Background())
	require.NoError(t, err)
	assert.Empty(t, data, "empty remote file should return empty byte slice, not an error")
}
