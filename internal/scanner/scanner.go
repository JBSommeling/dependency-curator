package scanner

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/JBSommeling/dependency-curator/internal/exec"
	"github.com/JBSommeling/dependency-curator/pkg/semver"
)

type Update struct {
	Name       string
	Current    string
	Wanted     string
	Latest     string
	UpdateType string // "patch", "minor", "major"
}

type Scanner struct {
	runner     exec.CommandRunner
	includeDev bool
}

func New(runner exec.CommandRunner, includeDev bool) *Scanner {
	return &Scanner{runner: runner, includeDev: includeDev}
}

// npmOutdatedEntry represents one entry from `npm outdated --json`
type npmOutdatedEntry struct {
	Current  string `json:"current"`
	Wanted   string `json:"wanted"`
	Latest   string `json:"latest"`
	Location string `json:"location"`
}

func (s *Scanner) ListAvailable(ctx context.Context, projectDir string) ([]Update, error) {
	args := []string{"outdated", "--json"}
	if !s.includeDev {
		args = append(args, "--omit=dev")
	}
	output, err := s.runner.RunAllowExit1(ctx, projectDir, "npm", args...)
	if err != nil {
		return nil, fmt.Errorf("running npm outdated: %w", err)
	}

	if len(output) == 0 {
		return nil, nil
	}

	var outdated map[string]npmOutdatedEntry
	if err := json.Unmarshal(output, &outdated); err != nil {
		return nil, fmt.Errorf("parsing npm outdated output: %w", err)
	}

	var updates []Update
	for name, entry := range outdated {
		if entry.Current == "" || entry.Latest == "" {
			continue
		}
		if entry.Current == entry.Latest {
			continue
		}

		current, err := semver.Parse(entry.Current)
		if err != nil {
			continue
		}
		latest, err := semver.Parse(entry.Latest)
		if err != nil {
			continue
		}

		updateType := semver.ClassifyUpdate(current, latest)
		if updateType == semver.None {
			continue
		}

		updates = append(updates, Update{
			Name:       name,
			Current:    entry.Current,
			Wanted:     entry.Wanted,
			Latest:     entry.Latest,
			UpdateType: string(updateType),
		})
	}

	return updates, nil
}
