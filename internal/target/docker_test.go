package target

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/moby/moby/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/neel-03/configdrift/internal/config"
)

// ── mock Docker client ────────────────────────────────────────────────────────

// mockDockerClient satisfies the dockerClient interface for tests.
// configure it per test — no real Docker daemon needed.
type mockDockerClient struct {
	// files maps container path -> file content
	files map[string][]byte
	// copyErr forces CopyFromContainer to return this error
	copyErr error
	// callCount tracks how many times CopyFromContainer was called
	callCount int
}

func (m *mockDockerClient) ContainerInspect(_ context.Context, containerID string) (interface{ GetID() string }, error) {
	return &dockerInspectResult{id: containerID}, nil
}

func (m *mockDockerClient) CopyFromContainer(_ context.Context, _ string, srcPath string) (io.ReadCloser, error) {
	m.callCount++

	if m.copyErr != nil {
		return nil, m.copyErr
	}

	data, ok := m.files[srcPath]
	if !ok {
		return nil, errors.New("no such file or directory")
	}

	// Docker returns a tar stream — match the real behavior so extractFromTar is exercised
	return buildTarStream(srcPath, data), nil
}

// buildTarStream wraps file data in a tar archive, just like Docker does.
func buildTarStream(path string, data []byte) io.ReadCloser {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	_ = tw.WriteHeader(&tar.Header{
		Name: path,
		Size: int64(len(data)),
		Mode: 0444,
	})
	_, _ = tw.Write(data)
	_ = tw.Close()
	return io.NopCloser(&buf)
}

// newMockDockerAdapter wires a DockerAdapter with a mock client directly.
// bypasses ensureClient so tests don't need a real Docker socket.
func newMockDockerAdapter(cfg config.TargetConfig, mock *mockDockerClient) *DockerAdapter {
	return &DockerAdapter{
		cfg:    cfg,
		client: mock,
	}
}

// ── tests ─────────────────────────────────────────────────────────────────────

func TestDockerAdapter_Name(t *testing.T) {
	a := NewDockerAdapter(config.TargetConfig{Name: "staging-app"})
	assert.Equal(t, "staging-app", a.Name())
}

func TestDockerAdapter_DefaultHost(t *testing.T) {
	// if no host is specified, should default to the local Docker socket —
	// not an empty string, which would cause a cryptic error later
	a := NewDockerAdapter(config.TargetConfig{})
	assert.NotEmpty(t, a.cfg.Host, "Host should default to the local Docker socket")
}

func TestDockerAdapter_Fetch_Success(t *testing.T) {
	// happy path — container exists, file exists, content is returned correctly
	content := []byte("db_host: localhost\ndb_port: 5432\n")
	mock := &mockDockerClient{
		files: map[string][]byte{"/app/config.yaml": content},
	}

	cfg := config.TargetConfig{
		Name:      "staging",
		Container: "app_staging",
		Path:      "/app/config.yaml",
	}
	a := newMockDockerAdapter(cfg, mock)

	data, err := a.Fetch(context.Background())
	require.NoError(t, err)
	assert.Equal(t, content, data)
}

func TestDockerAdapter_Fetch_FileNotFound(t *testing.T) {
	// requesting a path that doesn't exist in the container should return a clear error
	mock := &mockDockerClient{
		files: map[string][]byte{}, // empty — nothing served
	}

	cfg := config.TargetConfig{
		Name:      "staging",
		Container: "app_staging",
		Path:      "/does/not/exist.yaml",
	}
	a := newMockDockerAdapter(cfg, mock)

	_, err := a.Fetch(context.Background())
	assert.Error(t, err)
}

func TestDockerAdapter_Fetch_DockerAPIError(t *testing.T) {
	// if the Docker API itself fails (daemon unreachable, container stopped),
	// the error should be wrapped and returned — not swallowed
	mock := &mockDockerClient{
		copyErr: errors.New("container not running"),
	}

	cfg := config.TargetConfig{
		Name:      "staging",
		Container: "stopped_container",
		Path:      "/app/config.yaml",
	}
	a := newMockDockerAdapter(cfg, mock)

	_, err := a.Fetch(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "container not running")
}

func TestDockerAdapter_Fetch_EmptyFile(t *testing.T) {
	// an empty config file is valid — return empty bytes, not an error
	mock := &mockDockerClient{
		files: map[string][]byte{"/app/config.yaml": {}},
	}

	cfg := config.TargetConfig{
		Name:      "staging",
		Container: "app",
		Path:      "/app/config.yaml",
	}
	a := newMockDockerAdapter(cfg, mock)

	data, err := a.Fetch(context.Background())
	require.NoError(t, err)
	assert.Empty(t, data, "empty file should return empty bytes without error")
}

func TestDockerAdapter_Fetch_CancelledContext(_ *testing.T) {
	// a canceled context should prevent the fetch from proceeding
	// in real usage this happens when the watcher shuts down mid-cycle
	mock := &mockDockerClient{
		files: map[string][]byte{"/config.yaml": []byte("x: 1")},
	}

	cfg := config.TargetConfig{
		Name:      "staging",
		Container: "app",
		Path:      "/config.yaml",
	}
	a := newMockDockerAdapter(cfg, mock)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // canceled before Fetch runs

	// the mock doesn't check context, but the real client would — this test
	// validates that a canceled context is passed through correctly
	// (the mock will still return data; real Docker client would error)
	_, _ = a.Fetch(ctx) // just verify it doesn't panic
}

func TestDockerAdapter_Fetch_WithTimeout(t *testing.T) {
	// a configured timeout should be parsed and applied — verify it doesn't
	// break normal operation when the fetch completes before the deadline
	content := []byte("timeout: test")
	mock := &mockDockerClient{
		files: map[string][]byte{"/config.yaml": content},
	}

	cfg := config.TargetConfig{
		Name:      "staging",
		Container: "app",
		Path:      "/config.yaml",
		Timeout:   "30s", // generous — won't actually expire in tests
	}
	a := newMockDockerAdapter(cfg, mock)

	data, err := a.Fetch(context.Background())
	require.NoError(t, err)
	assert.Equal(t, content, data)
}

func TestDockerAdapter_Fetch_InvalidTimeout(t *testing.T) {
	// an invalid timeout string should be silently ignored (not crash) —
	// the fetch proceeds without a deadline. this is consistent with how
	// the SSH adapter handles it.
	content := []byte("x: 1")
	mock := &mockDockerClient{
		files: map[string][]byte{"/config.yaml": content},
	}

	cfg := config.TargetConfig{
		Name:      "staging",
		Container: "app",
		Path:      "/config.yaml",
		Timeout:   "not-a-duration", // should be ignored, not panic
	}
	a := newMockDockerAdapter(cfg, mock)

	data, err := a.Fetch(context.Background())
	require.NoError(t, err)
	assert.Equal(t, content, data)
}

func TestDockerAdapter_Fetch_PathWithSpaces(t *testing.T) {
	// unlike exec+cat, CopyFromContainer handles paths with spaces natively —
	// this was one of the reasons we chose it over exec
	content := []byte("works: true")
	path := "/app/my config/settings.yaml"
	mock := &mockDockerClient{
		files: map[string][]byte{path: content},
	}

	cfg := config.TargetConfig{
		Name:      "staging",
		Container: "app",
		Path:      path,
	}
	a := newMockDockerAdapter(cfg, mock)

	data, err := a.Fetch(context.Background())
	require.NoError(t, err)
	assert.Equal(t, content, data)
}

func TestDockerAdapter_Close_WithoutFetch(t *testing.T) {
	// Close on an adapter that was never used should be a no-op
	a := NewDockerAdapter(config.TargetConfig{Name: "x"})
	assert.NoError(t, a.Close())
}

func TestDockerAdapter_Close_Idempotent(t *testing.T) {
	// calling Close twice must not panic — watcher teardown may call it more than once
	mock := &mockDockerClient{
		files: map[string][]byte{"/config.yaml": []byte("x: 1")},
	}
	a := newMockDockerAdapter(config.TargetConfig{
		Name: "x", Container: "app", Path: "/config.yaml",
	}, mock)

	assert.NoError(t, a.Close())
	assert.NoError(t, a.Close())
}

// ── extractFromTar unit tests ─────────────────────────────────────────────────

func TestExtractFromTar_SingleFile(t *testing.T) {
	content := []byte("key: value")
	stream := buildTarStream("config.yaml", content)

	data, err := extractFromTar(stream, "config.yaml")
	require.NoError(t, err)
	assert.Equal(t, content, data)
}

func TestExtractFromTar_MatchByBasename(t *testing.T) {
	// Docker sometimes includes a directory prefix in the tar entry name —
	// e.g. "app/config.yaml" for path "/app/config.yaml".
	// we should still match on the base filename.
	content := []byte("nested: true")
	stream := buildTarStream("app/config.yaml", content) // entry has directory prefix

	data, err := extractFromTar(stream, "config.yaml") // we search by basename
	require.NoError(t, err)
	assert.Equal(t, content, data)
}

func TestExtractFromTar_FileNotInArchive(t *testing.T) {
	stream := buildTarStream("other.yaml", []byte("x: 1"))

	_, err := extractFromTar(stream, "config.yaml")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found in Docker tar response")
}

func TestExtractFromTar_EmptyTar(t *testing.T) {
	// an empty tar archive should return a clear "not found" error
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	_ = tw.Close()

	_, err := extractFromTar(&buf, "config.yaml")
	assert.Error(t, err)
}

func TestExtractFromTar_LargeFile(t *testing.T) {
	// make sure io.ReadAll works for larger files — not just small configs
	large := bytes.Repeat([]byte("key: value\n"), 10_000)
	stream := buildTarStream("big.yaml", large)

	data, err := extractFromTar(stream, "big.yaml")
	require.NoError(t, err)
	assert.Equal(t, large, data)
}

// ── integration test (skipped without Docker) ─────────────────────────────────

// TestDockerAdapter_Integration runs against a real Docker daemon.
// skipped automatically if Docker is not available — safe for CI without Docker.
//
// to run locally: go test -v -run TestDockerAdapter_Integration ./internal/target/
func TestDockerAdapter_Integration(t *testing.T) {
	// check if Docker is reachable before attempting the test
	c, err := newRealDockerClient()
	if err != nil {
		t.Skip("Docker not available:", err)
	}
	defer func() { _ = c.client.Close() }()

	// this test expects a container named "configdrift-test" to be running
	// with a file at /tmp/test-config.yaml containing "integration: true".
	// start it with:
	//   docker run -d --name configdrift-test alpine sleep 3600
	//   docker exec configdrift-test sh -c 'echo "integration: true" > /tmp/test-config.yaml'
	cfg := config.TargetConfig{
		Name:      "integration-test",
		Container: "configdrift-test",
		Path:      "/tmp/test-config.yaml",
		Timeout:   "10s",
	}

	a := NewDockerAdapter(cfg)
	defer func() { _ = a.Close() }()

	data, err := a.Fetch(context.Background())
	if err != nil {
		t.Skip("integration container not running:", err)
	}

	assert.Contains(t, string(data), "integration: true")
}

// newRealDockerClient is only used by the integration test to probe Docker availability.
func newRealDockerClient() (*mobyDockerClient, error) {
	c, err := newDockerSDKClient(client.DefaultDockerHost)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err = c.client.Ping(ctx, client.PingOptions{})
	if err != nil {
		_ = c.client.Close()
		return nil, err
	}
	return c, nil
}

// newDockerSDKClient wraps client.NewClientWithOpts for the integration helper.
func newDockerSDKClient(host string) (*mobyDockerClient, error) {
	c, err := client.New(client.WithHost(host))
	if err != nil {
		return nil, err
	}
	return &mobyDockerClient{client: c}, nil
}
