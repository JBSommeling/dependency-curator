package gomod

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/JBSommeling/dependency-curator/internal/dependency"
	"github.com/JBSommeling/dependency-curator/internal/exec"
)

type VulnScanner struct {
	runner exec.CommandRunner
}

func NewVulnScanner(runner exec.CommandRunner) *VulnScanner {
	return &VulnScanner{runner: runner}
}

type govulnMessage struct {
	OSV     *govulnOSV     `json:"osv"`
	Finding *govulnFinding `json:"finding"`
}

type govulnOSV struct {
	ID       string `json:"id"`
	Summary  string `json:"summary"`
	Affected []struct {
		Package struct {
			Name string `json:"name"`
		} `json:"package"`
		Ranges []struct {
			Events []struct {
				Introduced string `json:"introduced"`
				Fixed      string `json:"fixed"`
			} `json:"events"`
		} `json:"ranges"`
	} `json:"affected"`
}

type govulnFinding struct {
	OSV   string `json:"osv"`
	Trace []struct {
		Module  string `json:"module"`
		Version string `json:"version"`
	} `json:"trace"`
}

func (s *VulnScanner) Scan(ctx context.Context, projectDir string) ([]dependency.AdvisoryInfo, error) {
	output, err := s.runner.RunAllowExit1(ctx, projectDir, "govulncheck", "-json", "./...")
	if err != nil {
		if isNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("running govulncheck: %w", err)
	}

	if len(output) == 0 {
		return nil, nil
	}

	// Collect OSV metadata and findings
	osvMap := make(map[string]*govulnOSV)
	type findingInfo struct {
		module  string
		version string
	}
	findings := make(map[string]findingInfo)

	decoder := json.NewDecoder(strings.NewReader(string(output)))
	for decoder.More() {
		var msg govulnMessage
		if err := decoder.Decode(&msg); err != nil {
			continue
		}

		if msg.OSV != nil {
			osvMap[msg.OSV.ID] = msg.OSV
		}

		if msg.Finding != nil && len(msg.Finding.Trace) > 0 {
			trace := msg.Finding.Trace[0]
			findings[msg.Finding.OSV] = findingInfo{
				module:  trace.Module,
				version: trace.Version,
			}
		}
	}

	var advisories []dependency.AdvisoryInfo
	for osvID, info := range findings {
		osv, ok := osvMap[osvID]
		if !ok {
			continue
		}

		var affectedRange, fixedVersion string
		if len(osv.Affected) > 0 && len(osv.Affected[0].Ranges) > 0 {
			for _, event := range osv.Affected[0].Ranges[0].Events {
				if event.Introduced != "" {
					affectedRange = ">=" + event.Introduced
				}
				if event.Fixed != "" {
					fixedVersion = event.Fixed
				}
			}
		}

		advisories = append(advisories, dependency.AdvisoryInfo{
			ID:               osvID,
			Package:          info.module,
			Severity:         "unknown",
			Title:            osv.Summary,
			AffectedVersions: affectedRange,
			FixedVersion:     stripVersionPrefix(fixedVersion),
			URL:              "https://pkg.go.dev/vuln/" + osvID,
		})
	}

	return advisories, nil
}

func isNotFound(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "not found") ||
		strings.Contains(msg, "executable file not found") ||
		strings.Contains(msg, "no such file or directory")
}
