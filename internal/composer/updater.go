package composer

import (
	"context"
	"fmt"
	"strings"

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
	var targets []dependency.Dependency
	for _, d := range deps {
		if d.LatestVersion != "" {
			targets = append(targets, d)
		}
	}

	if len(targets) == 0 {
		return nil, nil
	}

	var errs []string
	var applied []dependency.Dependency

	for _, d := range targets {
		constraint := d.Name + ":" + d.LatestVersion
		_, err := u.runner.Run(ctx, projectDir, "composer", "update", "--with", constraint)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", d.Name, err))
			continue
		}
		applied = append(applied, d)
	}

	if len(errs) > 0 {
		return applied, fmt.Errorf("some updates failed: %s", strings.Join(errs, "; "))
	}

	return applied, nil
}

func (u *Updater) ApplyPatches(ctx context.Context, projectDir string, deps []dependency.Dependency) ([]dependency.Dependency, error) {
	var patches []dependency.Dependency
	for _, d := range deps {
		if d.UpdateType == "patch" {
			patches = append(patches, d)
		}
	}
	return u.ApplyUpdates(ctx, projectDir, patches)
}
