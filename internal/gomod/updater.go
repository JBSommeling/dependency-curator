package gomod

import (
	"context"
	"fmt"
	"log"

	"github.com/JBSommeling/dependency-curator/internal/dependency"
	"github.com/JBSommeling/dependency-curator/internal/exec"
)

type Updater struct {
	runner exec.CommandRunner
}

func NewUpdater(runner exec.CommandRunner) *Updater {
	return &Updater{runner: runner}
}

func (u *Updater) ApplyPatches(ctx context.Context, projectDir string, deps []dependency.Dependency) ([]dependency.Dependency, error) {
	var applied []dependency.Dependency

	for _, dep := range deps {
		if dep.UpdateType != "patch" {
			continue
		}

		target := dep.Name + "@v" + dep.LatestVersion
		if _, err := u.runner.Run(ctx, projectDir, "go", "get", target); err != nil {
			log.Printf("warning: failed to update %s: %v", dep.Name, err)
			return applied, fmt.Errorf("updating %s: %w", dep.Name, err)
		}
		applied = append(applied, dep)
	}

	if len(applied) > 0 {
		if _, err := u.runner.Run(ctx, projectDir, "go", "mod", "tidy"); err != nil {
			return nil, fmt.Errorf("running go mod tidy: %w", err)
		}
	}

	return applied, nil
}
