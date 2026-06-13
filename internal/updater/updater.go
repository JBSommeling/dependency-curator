package updater

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

func New(runner exec.CommandRunner) *Updater {
	return &Updater{runner: runner}
}

func (u *Updater) ApplyPatches(ctx context.Context, projectDir string, deps []dependency.Dependency) ([]dependency.Dependency, error) {
	var patches []dependency.Dependency
	for _, d := range deps {
		if d.UpdateType == "patch" && d.LatestVersion != "" {
			patches = append(patches, d)
		}
	}

	if len(patches) == 0 {
		return nil, nil
	}

	var errs []string
	var applied []dependency.Dependency

	for _, d := range patches {
		pkg := d.Name + "@" + d.LatestVersion
		_, err := u.runner.Run(ctx, projectDir, "npm", "install", "--", pkg)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", d.Name, err))
			continue
		}
		applied = append(applied, d)
	}

	if len(errs) > 0 {
		return applied, fmt.Errorf("some patches failed: %s", strings.Join(errs, "; "))
	}

	return applied, nil
}
