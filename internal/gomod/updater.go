package gomod

import (
	"context"
	"fmt"

	"github.com/JBSommeling/dependency-curator/internal/dependency"
	"github.com/JBSommeling/dependency-curator/internal/exec"
)

type Updater struct {
	runner exec.CommandRunner
}

func NewUpdater(runner exec.CommandRunner) *Updater {
	return &Updater{runner: runner}
}

func (u *Updater) ApplyUpdates(ctx context.Context, projectDir string, deps []dependency.Dependency) ([]dependency.Dependency, error) {
	var applied []dependency.Dependency
	var firstErr error

	for _, dep := range deps {
		if dep.LatestVersion == "" {
			continue
		}
		target := dep.Name + "@v" + dep.LatestVersion
		if _, err := u.runner.Run(ctx, projectDir, "go", "get", target); err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("updating %s: %w", dep.Name, err)
			}
			continue
		}
		applied = append(applied, dep)
	}

	if len(applied) > 0 {
		if _, err := u.runner.Run(ctx, projectDir, "go", "mod", "tidy"); err != nil {
			return applied, fmt.Errorf("running go mod tidy: %w", err)
		}
	}

	return applied, firstErr
}

func (u *Updater) ApplyPatches(ctx context.Context, projectDir string, deps []dependency.Dependency) ([]dependency.Dependency, error) {
	var patches []dependency.Dependency
	for _, dep := range deps {
		if dep.UpdateType == "patch" {
			patches = append(patches, dep)
		}
	}
	return u.ApplyUpdates(ctx, projectDir, patches)
}
