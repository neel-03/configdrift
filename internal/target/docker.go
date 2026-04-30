package target

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"sync"
	"time"

	"github.com/moby/moby/client"
	"github.com/neel-03/configdrift/internal/config"
)

// DockerAdapter implements the Adapter interface for
// Docker container targets, it reads a file from a running container
// using the Docker API's CopyFromContainer method.
type DockerAdapter struct {
	cfg    config.TargetConfig
	client dockerClient
	mu     sync.Mutex // guards cli - lazy init on first Fetch
}

// NewDockerAdapter creates a Docker adapter.
// the actual Docker client connection is deferred to the first Fetch call.
func NewDockerAdapter(cfg config.TargetConfig) *DockerAdapter {
	// default to the local Docker socket if the user didn't specify a host.
	// this covers the most common case: Docker running on the same machine.
	if cfg.Host == "" {
		slog.Info("No Docker host specified, using default Docker socket", "default_host", client.DefaultDockerHost)
		cfg.Host = client.DefaultDockerHost
	}
	return &DockerAdapter{cfg: cfg}
}

// Name returns the target name.
func (a *DockerAdapter) Name() string {
	return a.cfg.Name
}

// Fetch reads the config file from inside the container using CopyFromContainer.
// CopyFromContainer returns a tar archive containing the requested file, we
// unwrap it here. this avoids any shell involvement and handles filenames with
// spaces or special characters correctly.
func (a *DockerAdapter) Fetch(ctx context.Context) ([]byte, error) {
	if err := a.ensureDockerClient(); err != nil {
		return nil, err
	}

	// apply the timeout if it was specified in the config.
	// this makes sure we don't hang indefinitely if the container is unresponsive.
	fetchCtx, cancel := a.applyTimeout(ctx)
	if cancel != nil {
		defer cancel()
	}

	// once timeout is applied, we can call the CopyFromContainer method.
	tarStream, err := a.client.CopyFromContainer(fetchCtx, a.cfg.Container, a.cfg.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to copy file from container %q: %w", a.cfg.Container, err)
	}
	defer tarStream.Close()

	return extractFromTar(tarStream, filepath.Base(a.cfg.Path))
}

// ensureDockerClient lazily initialises the Docker SDK client on first use.
// we don't connect in NewDockerAdapter because the docker daemon might not be
// running at startup.
func (a *DockerAdapter) ensureDockerClient() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.client != nil {
		return nil // already initialized
	}

	// we are not specifying any deamon version, it should
	// be able to negotiate the best version automatically.
	c, err := client.New(client.WithHost(a.cfg.Host))

	if err != nil {
		return fmt.Errorf("failed to create Docker client for host %q: %w", a.cfg.Host, err)
	}

	a.client = &mobyDockerClient{client: c}

	return nil
}

// applyTimeout applies the configured timeout to the context if specified.
func (a *DockerAdapter) applyTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if a.cfg.Timeout == "" {
		return ctx, nil
	}
	duration, err := time.ParseDuration(a.cfg.Timeout)
	if err != nil {
		slog.Warn("Ignoring invalid timeout", "target", a.Name(),
			"invalid_timeout", a.cfg.Timeout, "error", err)
		return ctx, nil
	}
	return context.WithTimeout(ctx, duration)
}

// Close releases the underlying Docker client.
// safe to call even if Fetch was never called.
func (a *DockerAdapter) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if moby, ok := a.client.(*mobyDockerClient); ok && moby.client != nil {
		return moby.client.Close()
	}
	return nil
}
