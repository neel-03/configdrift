package target

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/neel-03/configdrift/internal/config"
)

// K8sAdapter implements the Adapter interface for Kubernetes ConfigMap targets.
// it reads a specific key from a ConfigMap using the K8s API
type K8sAdapter struct {
	cfg    config.TargetConfig
	client kubernetes.Interface
}

// NewK8sAdapter creates a K8s adapter and eagerly builds the clientset.
// we connect at construction time (unlike SSH/Docker which are lazy) because
// kubeconfig loading is cheap and it's better to fail fast at startup than
// on the first poll cycle.
func NewK8sAdapter(cfg config.TargetConfig) (*K8sAdapter, error) {
	client, err := buildClientset(cfg.KubeConfig)
	if err != nil {
		return nil, err
	}
	return &K8sAdapter{cfg: cfg, client: client}, nil
}

// Name returns the target name.
func (a *K8sAdapter) Name() string {
	return a.cfg.Name
}

// Fetch reads the value of [cfg.CMKey] from the ConfigMap [cfg.ConfigMap]
// in namespace [cfg.Namespace].
func (a *K8sAdapter) Fetch(ctx context.Context) ([]byte, error) {
	// apply the timeout if it was specified in the config.
	// this makes sure we don't hang indefinitely if the container is unresponsive.
	fetchCtx, cancel := a.applyTimeout(ctx)
	if cancel != nil {
		defer cancel()
	}
	configMap, err := a.client.CoreV1().
		ConfigMaps(a.cfg.Namespace).
		Get(fetchCtx, a.cfg.ConfigMap, v1.GetOptions{})
	if err != nil {
		return nil, err
	}

	// [configMap.Data] is map[string]string, check it exists first
	if configMap.Data == nil {
		return nil, fmt.Errorf("ConfigMap %q in namespace %q has no data",
			a.cfg.ConfigMap, a.cfg.Namespace)
	}

	value, ok := configMap.Data[a.cfg.CMKey]
	if !ok {
		// show what keys are present for debugging
		keys := make([]string, 0, len(configMap.Data))
		for k := range configMap.Data {
			keys = append(keys, k)
		}
		return nil, fmt.Errorf("key %q not found in ConfigMap %q (available keys: %v)",
			a.cfg.CMKey, a.cfg.ConfigMap, keys)
	}

	return []byte(value), nil
}

// applyTimeout applies the configured timeout to the context if specified.
func (a *K8sAdapter) applyTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
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
