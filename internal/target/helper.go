package target

import (
	"archive/tar"
	"context"
	"fmt"
	"io"
	"path/filepath"

	"github.com/moby/moby/client"
)

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
