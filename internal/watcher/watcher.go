package watcher

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/neel-03/configdrift/internal/config"
	"github.com/neel-03/configdrift/internal/diff"
	"github.com/neel-03/configdrift/internal/parser"
	"github.com/neel-03/configdrift/internal/source"
	"github.com/neel-03/configdrift/internal/target"
)

// Watcher orchestrates the drift detection cycle.
type Watcher struct {
	cfg      *config.Policy
	source   source.Source
	adapters []target.Adapter
}

// FromPolicy creates a new Watcher based on the provided policy.
// It initializes all sources and adapters.
func FromPolicy(ctx context.Context, cfg *config.Policy) (*Watcher, error) {
	var src source.Source
	var err error

	switch cfg.Canonical.Type {
	case config.TypeLocal:
		src = source.NewLocalSource(cfg.Canonical.Path)
	case config.TypeGit:
		src = source.NewGitSource(
			cfg.Canonical.Repo,
			cfg.Canonical.Branch,
			cfg.Canonical.Path,
			cfg.Canonical.AuthToken,
		)
	case config.TypeS3:
		src, err = source.NewS3Source(
			ctx,
			cfg.Canonical.S3Bucket,
			cfg.Canonical.Path,
			cfg.Canonical.S3Region,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to init s3 source: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported source type: %s", cfg.Canonical.Type)
	}

	adapters := make([]target.Adapter, 0, len(cfg.Targets))
	for _, targetCfg := range cfg.Targets {
		var adapter target.Adapter
		switch targetCfg.Type {
		case config.TypeSSH:
			adapter = target.NewSSHAdapter(targetCfg)
		case config.TypeDocker:
			adapter = target.NewDockerAdapter(targetCfg)
		case config.TypeK8s:
			adapter, err = target.NewK8sAdapter(targetCfg)
			if err != nil {
				return nil, fmt.Errorf("failed to create k8s adapter for %s: %w", targetCfg.Name, err)
			}
		default:
			return nil, fmt.Errorf("unsupported target type: %s", targetCfg.Type)
		}
		adapters = append(adapters, adapter)
	}

	return NewWatcher(cfg, src, adapters), nil
}

// NewWatcher creates a new Watcher based on the provided policy and initialized components.
func NewWatcher(cfg *config.Policy, src source.Source, adapters []target.Adapter) *Watcher {
	return &Watcher{
		cfg:      cfg,
		source:   src,
		adapters: adapters,
	}
}

// RunCycle performs a single drift detection cycle for all targets.
func (w *Watcher) RunCycle(ctx context.Context) ([]diff.DriftResult, error) {
	slog.Debug("Starting drift detection cycle")

	// 1. fetch and parse canonical data
	canonicalData, err := w.source.Fetch(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch canonical data: %w", err)
	}

	p, err := parser.FromPath(w.cfg.Canonical.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to get canonical parser: %w", err)
	}

	parsed, err := p.Parse(canonicalData)
	if err != nil {
		return nil, fmt.Errorf("failed to parse canonical data: %w", err)
	}

	canonicalFlattened := diff.Flatten(parsed)

	// 2. process each target in parallel
	g, gctx := errgroup.WithContext(ctx)
	resultsChan := make(chan diff.DriftResult, len(w.adapters))

	for i, adapter := range w.adapters {
		a := adapter
		targetCfg := w.cfg.Targets[i] // assumes adapters match targets 1:1 in order
		g.Go(func() error {
			res, err := w.checkTarget(gctx, targetCfg, a, canonicalFlattened)
			if err != nil {
				slog.Error("Failed to check target", "target", a.Name(), "error", err)
				return nil
			}
			resultsChan <- res
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, fmt.Errorf("parallel check failed: %w", err)
	}
	close(resultsChan)

	var results []diff.DriftResult
	for res := range resultsChan {
		results = append(results, res)
	}

	return results, nil
}

// checkTarget performs drift detection for a single target.
func (w *Watcher) checkTarget(ctx context.Context, targetCfg config.TargetConfig, adapter target.Adapter, canonicalFlattened map[string]interface{}) (diff.DriftResult, error) {
	// Apply per-target timeout if specified
	if targetCfg.Timeout != "" {
		duration, err := time.ParseDuration(targetCfg.Timeout)
		if err == nil {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, duration)
			defer cancel()
		} else {
			slog.Warn("Ignoring invalid timeout duration", "target", targetCfg.Name, "timeout", targetCfg.Timeout, "error", err)
		}
	}

	targetData, err := adapter.Fetch(ctx)
	if err != nil {
		return diff.DriftResult{}, fmt.Errorf("failed to fetch target data: %w", err)
	}

	tp, err := parser.FromPath(targetCfg.Path)
	if err != nil {
		return diff.DriftResult{}, fmt.Errorf("failed to get target parser: %w", err)
	}

	targetParsed, err := tp.Parse(targetData)
	if err != nil {
		return diff.DriftResult{}, fmt.Errorf("failed to parse target data: %w", err)
	}

	targetFlattened := diff.Flatten(targetParsed)
	return diff.Compare(targetCfg.Name, canonicalFlattened, targetFlattened), nil
}

// Run starts a continuous watch loop.
func (w *Watcher) Run(ctx context.Context) error {
	interval, _ := time.ParseDuration(w.cfg.Interval)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	slog.Info("Starting watcher", "interval", w.cfg.Interval)

	for {
		results, err := w.RunCycle(ctx)
		if err != nil {
			slog.Error("Cycle failed", "error", err)
		} else {
			for _, res := range results {
				if res.IsDrifted {
					slog.Warn("Drift detected", "target", res.TargetName)
					// TODO: Fan out to Alerters
				}
			}
		}

		select {
		case <-ctx.Done():
			slog.Info("Watcher stopping")
			return ctx.Err()
		case <-ticker.C:
			continue
		}
	}
}
