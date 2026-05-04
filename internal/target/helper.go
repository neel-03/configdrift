package target

import (
	"archive/tar"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/moby/moby/client"
	"github.com/neel-03/configdrift/internal/utils"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// -- Docker Utils --

// dockerClient is a minimal interface over the Docker SDK client.
// we only need two operations:
//  1. inspect (to verify the container exists)
//  2. copy (to read a file).
//
// defining our own interface here makes the adapter trivially
// mockable in tests without a real Docker daemon.
type dockerClient interface {
	ContainerInspect(ctx context.Context, containerID string) (interface{ GetID() string }, error)
	CopyFromContainer(ctx context.Context, containerID, srcPath string) (io.ReadCloser, error)
}

type mobyDockerClient struct {
	client *client.Client
}

func (moby *mobyDockerClient) ContainerInspect(ctx context.Context, containerID string) (interface{ GetID() string }, error) {
	info, err := moby.client.ContainerInspect(
		ctx,
		containerID,
		client.ContainerInspectOptions{},
	)
	if err != nil {
		return nil, err
	}
	return &dockerInspectResult{id: info.Container.ID}, nil
}

func (moby *mobyDockerClient) CopyFromContainer(ctx context.Context, containerID, srcPath string) (io.ReadCloser, error) {
	options := client.CopyFromContainerOptions{SourcePath: srcPath}
	result, err := moby.client.CopyFromContainer(ctx, containerID, options)
	if err != nil {
		return nil, err
	}
	return result.Content, nil
}

type dockerInspectResult struct{ id string }

func (res *dockerInspectResult) GetID() string {
	return res.id
}

// extractFromTar reads the first matching file entry from a tar stream.
// Docker always wraps CopyFromContainer results in a single-file tar, but we
// match by filename defensively in case that ever changes.
func extractFromTar(reader io.Reader, filename string) ([]byte, error) {
	tr := tar.NewReader(reader)

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break // no more entries
		}
		if err != nil {
			return nil, err
		}

		// Docker sometimes prefixes the entry name with a directory component,
		// match on the base filename only to be safe
		if filepath.Base(hdr.Name) == filename || hdr.Name == filename {
			data, err := io.ReadAll(tr)
			if err != nil {
				return nil, err
			}
			return data, nil
		}
	}

	return nil, fmt.Errorf("file %q not found in Docker tar response", filename)
}

// -- K8s Utils --

// buildClientset constructs a [kubernetes.Clientset] from the given kubeconfig path.
// resolution order (mirrors kubectl behaviour):
//  1. explicit kubeConfigPath arg (from targets.yaml kubeconfig field)
//  2. KUBECONFIG environment variable
//  3. ~/.kube/config (default kubeconfig location)
//  4. in-cluster config (when running inside a Pod - service account token)
//
// this means configdrift works correctly both when run locally by a developer
// and when deployed as a Pod inside the cluster it's monitoring.
func buildClientset(kubeConfigPath string) (kubernetes.Interface, error) {
	restCfg, err := buildRestCfg(kubeConfigPath)
	if err != nil {
		return nil, err
	}

	clientSet, err := kubernetes.NewForConfig(restCfg)
	if err != nil {
		return nil, err
	}

	return clientSet, nil
}

// buildRestCfg builds a [rest.Config] from the given kubeconfig path.
func buildRestCfg(kubeConfigPath string) (*rest.Config, error) {
	// case 1: explicit path from config
	if kubeConfigPath != "" {
		slog.Info("Reading kubeconfig from explicit path", "path", kubeConfigPath)
		return createRestConfigFromPath(kubeConfigPath)
	}

	// case 2: KUBECONFIG env var
	if env := os.Getenv("KUBECONFIG"); env != "" {
		slog.Info("Reading kubeconfig from KUBECONFIG env var", "path", env)
		return createRestConfigFromPath(env)
	}

	// case 3: default kubeconfig location
	defaultKubeConfig := utils.ExpandPath("~/.kube/config")
	if _, err := os.Stat(defaultKubeConfig); err == nil {
		slog.Info("Reading kubeconfig from default location", "path", defaultKubeConfig)
		return createRestConfigFromPath(defaultKubeConfig)
	}

	// case 4: in-cluster config (when running inside a Pod)
	slog.Info("Attempting in-cluster configuration")
	cfg, err := rest.InClusterConfig()
	if err == nil {
		return cfg, nil
	}

	return nil, fmt.Errorf("could not load kubeconfig (tried explicit path, env var, default location, and in-cluster): %w", err)
}

// createRestConfigFromPath creates a [rest.Config] from the given kubeconfig path.
// it expands ~ in the path and uses clientcmd.BuildConfigFromFlags
// to build the rest.Config.
func createRestConfigFromPath(path string) (*rest.Config, error) {
	expandedPath := utils.ExpandPath(path)
	cfg, err := clientcmd.BuildConfigFromFlags("", expandedPath)
	if err != nil {
		return nil, err
	}
	return cfg, nil
}
