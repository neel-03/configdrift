package target

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildRestCfg_ResolutionOrder(t *testing.T) {
	tmpDir := t.TempDir()

	// Create dummy kubeconfig files
	pathExplicit := filepath.Join(tmpDir, "explicit.config")
	pathEnv := filepath.Join(tmpDir, "env.config")

	err := os.WriteFile(pathExplicit, []byte("invalid config content"), 0600)
	assert.NoError(t, err)
	err = os.WriteFile(pathEnv, []byte("invalid config content"), 0600)
	assert.NoError(t, err)

	t.Run("Explicit path takes precedence over Env Var", func(t *testing.T) {
		// Set env var but also provide explicit path
		t.Setenv("KUBECONFIG", pathEnv)

		cfg, err := buildRestCfg(pathExplicit)

		// It should fail to parse, but we want to see WHICH file it tried to parse
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "explicit.config")
		assert.NotContains(t, err.Error(), "env.config")
		assert.Nil(t, cfg)
	})

	t.Run("Env Var takes precedence if no explicit path", func(t *testing.T) {
		t.Setenv("KUBECONFIG", pathEnv)

		cfg, err := buildRestCfg("")

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "env.config")
		assert.Nil(t, cfg)
	})

	t.Run("Error message lists all tried methods", func(t *testing.T) {
		// Clear env and provide no path
		t.Setenv("KUBECONFIG", "")

		// Ensure default ~/.kube/config doesn't exist for the test (hard to do perfectly, but we can check the final error)
		// We'll also mock the "home" so it doesn't find a real one
		t.Setenv("HOME", tmpDir)

		cfg, err := buildRestCfg("")

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "tried explicit path, env var, default location, and in-cluster")
		assert.Nil(t, cfg)
	})
}
