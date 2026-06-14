package gomod

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/JBSommeling/dependency-curator/internal/dependency"
	"github.com/JBSommeling/dependency-curator/internal/exec"
	"github.com/JBSommeling/dependency-curator/pkg/semver"
)

type Scanner struct {
	runner exec.CommandRunner
}

func NewScanner(runner exec.CommandRunner) *Scanner {
	return &Scanner{runner: runner}
}

// goListModule represents one entry from `go list -m -u -json all`
type goListModule struct {
	Path     string        `json:"Path"`
	Version  string        `json:"Version"`
	Indirect bool          `json:"Indirect"`
	Update   *goListUpdate `json:"Update"`
	Main     bool          `json:"Main"`
}

type goListUpdate struct {
	Version string `json:"Version"`
}

func (s *Scanner) ListAvailable(ctx context.Context, projectDir string) ([]dependency.UpdateInfo, error) {
	output, err := s.runner.Run(ctx, projectDir, "go", "list", "-m", "-u", "-json", "all")
	if err != nil {
		return nil, fmt.Errorf("running go list: %w", err)
	}

	if len(output) == 0 {
		return nil, nil
	}

	// go list -m -json outputs concatenated JSON objects, not an array
	decoder := json.NewDecoder(strings.NewReader(string(output)))
	var updates []dependency.UpdateInfo

	for decoder.More() {
		var mod goListModule
		if err := decoder.Decode(&mod); err != nil {
			return nil, fmt.Errorf("parsing go list output: %w", err)
		}

		// Skip main module and indirect deps
		if mod.Main || mod.Indirect {
			continue
		}

		// Only include modules with available updates
		if mod.Update == nil {
			continue
		}

		current := stripVersionPrefix(mod.Version)
		latest := stripVersionPrefix(mod.Update.Version)

		currentVer, err := semver.Parse(current)
		if err != nil {
			continue
		}
		latestVer, err := semver.Parse(latest)
		if err != nil {
			continue
		}

		updateType := semver.ClassifyUpdate(currentVer, latestVer)

		updates = append(updates, dependency.UpdateInfo{
			Name:       mod.Path,
			Current:    current,
			Wanted:     latest,
			Latest:     latest,
			UpdateType: string(updateType),
		})
	}

	return updates, nil
}

// stripVersionPrefix removes the leading "v" from a Go module version string.
func stripVersionPrefix(v string) string {
	return strings.TrimPrefix(v, "v")
}
