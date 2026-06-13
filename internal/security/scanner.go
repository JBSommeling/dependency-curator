package security

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/JBSommeling/dependency-curator/internal/exec"
)

type Advisory struct {
	ID               string
	Package          string
	Severity         string
	Title            string
	AffectedVersions string
	FixedVersion     string
	URL              string
}

type Scanner interface {
	Scan(ctx context.Context, projectDir string) ([]Advisory, error)
}

type NpmAuditScanner struct {
	runner     exec.CommandRunner
	includeDev bool
}

func NewNpmAuditScanner(runner exec.CommandRunner, includeDev bool) *NpmAuditScanner {
	return &NpmAuditScanner{runner: runner, includeDev: includeDev}
}

// npm audit --json v2 format
type npmAuditOutput struct {
	Vulnerabilities map[string]npmVulnerability `json:"vulnerabilities"`
}

type npmVulnerability struct {
	Name         string            `json:"name"`
	Severity     string            `json:"severity"`
	Via          []json.RawMessage `json:"via"`
	FixAvailable interface{}       `json:"fixAvailable"`
	Range        string            `json:"range"`
}

type npmViaEntry struct {
	Source   int    `json:"source"`
	Name     string `json:"name"`
	Title    string `json:"title"`
	URL      string `json:"url"`
	Severity string `json:"severity"`
	Range    string `json:"range"`
}

func (s *NpmAuditScanner) Scan(ctx context.Context, projectDir string) ([]Advisory, error) {
	args := []string{"audit", "--json"}
	if !s.includeDev {
		args = append(args, "--omit=dev")
	}
	output, err := s.runner.RunAllowExit1(ctx, projectDir, "npm", args...)
	if err != nil {
		return nil, fmt.Errorf("running npm audit: %w", err)
	}

	if len(output) == 0 {
		return nil, nil
	}

	var audit npmAuditOutput
	if err := json.Unmarshal(output, &audit); err != nil {
		return nil, fmt.Errorf("parsing npm audit output: %w", err)
	}

	var advisories []Advisory

	for pkgName, vuln := range audit.Vulnerabilities {
		for _, raw := range vuln.Via {
			// "via" can be either an object (direct vulnerability) or a string (transitive)
			var entry npmViaEntry
			if err := json.Unmarshal(raw, &entry); err != nil {
				// It's a string reference to another package — skip
				continue
			}
			// Only process actual advisory objects (have a source ID)
			if entry.Source == 0 && entry.Title == "" {
				continue
			}

			advisories = append(advisories, Advisory{
				ID:               strconv.Itoa(entry.Source),
				Package:          pkgName,
				Severity:         normalizeSeverity(entry.Severity),
				Title:            entry.Title,
				AffectedVersions: entry.Range,
				URL:              entry.URL,
			})
		}
	}

	return advisories, nil
}

func normalizeSeverity(s string) string {
	switch s {
	case "critical", "high", "moderate", "low", "info":
		return s
	default:
		return "unknown"
	}
}
