package watcher

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/neel-03/configdrift/internal/config"
	"github.com/neel-03/configdrift/internal/target"
)

type mockSource struct {
	data []byte
	err  error
}

func (s *mockSource) Fetch(ctx context.Context) ([]byte, error) {
	return s.data, s.err
}

func (s *mockSource) String() string {
	return "mock"
}

type mockAdapter struct {
	name string
	data []byte
	err  error
}

func (a *mockAdapter) Name() string {
	return a.name
}

func (a *mockAdapter) Fetch(ctx context.Context) ([]byte, error) {
	return a.data, a.err
}

func TestWatcher_RunCycle(t *testing.T) {
	cfg := &config.Policy{
		Canonical: config.CanonicalConfig{
			Type: config.TypeLocal,
			Path: "test.yaml",
		},
		Targets: []config.TargetConfig{
			{
				Name: "target1",
				Type: config.TypeSSH,
				Path: "target.yaml",
			},
		},
	}

	src := &mockSource{
		data: []byte("key: value"),
	}
	adapters := []target.Adapter{
		&mockAdapter{
			name: "target1",
			data: []byte("key: drifted"),
		},
	}

	w := NewWatcher(cfg, src, adapters, nil)
	results, err := w.RunCycle(context.Background())

	assert.NoError(t, err)
	assert.Len(t, results, 1)
	assert.True(t, results[0].IsDrifted)
	assert.Equal(t, "target1", results[0].TargetName)
}

func TestWatcher_FromPolicy_InvalidType(t *testing.T) {
	cfg := &config.Policy{
		Canonical: config.CanonicalConfig{
			Type: "invalid",
		},
	}
	w, err := FromPolicy(context.Background(), cfg, nil)
	assert.Error(t, err)
	assert.Nil(t, w)
	assert.Contains(t, err.Error(), "unsupported source type")
}
