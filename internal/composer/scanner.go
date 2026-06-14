package composer

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/JBSommeling/dependency-curator/internal/dependency"
	"github.com/JBSommeling/dependency-curator/internal/exec"
	"github.com/JBSommeling/dependency-curator/pkg/semver"
)

type Scanner struct {
	runner     exec.CommandRunner
	includeDev bool
}

func NewScanner(runner exec.CommandRunner, includeDev bool) *Scanner {
	return &Scanner{runner: runner, includeDev: includeDev}
}

type composerOutdatedOutput struct {
	Installed []composerOutdatedEntry `json:"installed"`
}

type composerOutdatedEntry struct {
	Name         string `json:"name"`
	Version      string `json:"version"`
	Latest       string `json:"latest"`
	LatestStatus string `json:"latest-status"`
	Description  string `json:"description"`
}

func (s *Scanner) ListAvailable(ctx context.Context, projectDir string) ([]dependency.UpdateInfo, error) {
	args := []string{"outdated", "--format=json", "--direct"}
	if !s.includeDev {
		args = append(args, "--no-dev")
	}

	output, err := s.runner.RunAllowExit1(ctx, projectDir, "composer", args...)
	if err != nil {
		return nil, fmt.Errorf("running composer outdated: %w", err)
	}

	if len(output) == 0 {
		return nil, nil
	}

	var outdated composerOutdatedOutput
	if err := json.Unmarshal(output, &outdated); err != nil {
		return nil, fmt.Errorf("parsing composer outdated output: %w", err)
	}

	var updates []dependency.UpdateInfo
	for _, entry := range outdated.Installed {
		if entry.Version == "" || entry.Latest == "" {
			continue
		}
		if entry.Version == entry.Latest {
			continue
		}

		// Strip 'v' prefix common in Composer versions
		currentStr := cleanVersion(entry.Version)
		latestStr := cleanVersion(entry.Latest)

		current, err := semver.Parse(currentStr)
		if err != nil {
			continue
		}
		latest, err := semver.Parse(latestStr)
		if err != nil {
			continue
		}

		updateType := semver.ClassifyUpdate(current, latest)
		if updateType == semver.None {
			continue
		}

		wanted := currentStr
		if entry.LatestStatus == "semver-safe-update" {
			wanted = latestStr
		}

		updates = append(updates, dependency.UpdateInfo{
			Name:       entry.Name,
			Current:    currentStr,
			Wanted:     wanted,
			Latest:     latestStr,
			UpdateType: string(updateType),
		})
	}

	return updates, nil
}
