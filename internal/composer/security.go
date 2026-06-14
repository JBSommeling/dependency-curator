package composer

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/JBSommeling/dependency-curator/internal/dependency"
	"github.com/JBSommeling/dependency-curator/internal/exec"
)

type AuditScanner struct {
	runner     exec.CommandRunner
	includeDev bool
}

func NewAuditScanner(runner exec.CommandRunner, includeDev bool) *AuditScanner {
	return &AuditScanner{runner: runner, includeDev: includeDev}
}

type composerAuditOutput struct {
	Advisories map[string][]composerAdvisory `json:"advisories"`
}

type composerAdvisory struct {
	AdvisoryID       string `json:"advisoryId"`
	PackageName      string `json:"packageName"`
	Title            string `json:"title"`
	Link             string `json:"link"`
	CVE              string `json:"cve"`
	AffectedVersions string `json:"affectedVersions"`
	Sources          []struct {
		Name     string `json:"name"`
		RemoteID string `json:"remoteId"`
	} `json:"sources"`
}

func (s *AuditScanner) Scan(ctx context.Context, projectDir string) ([]dependency.AdvisoryInfo, error) {
	args := []string{"audit", "--format=json"}
	if !s.includeDev {
		args = append(args, "--no-dev")
	}

	output, err := s.runner.RunAllowExit1(ctx, projectDir, "composer", args...)
	if err != nil {
		return nil, fmt.Errorf("running composer audit: %w", err)
	}

	if len(output) == 0 {
		return nil, nil
	}

	var audit composerAuditOutput
	if err := json.Unmarshal(output, &audit); err != nil {
		return nil, fmt.Errorf("parsing composer audit output: %w", err)
	}

	var advisories []dependency.AdvisoryInfo

	for pkgName, advs := range audit.Advisories {
		for _, a := range advs {
			id := a.AdvisoryID
			if id == "" && a.CVE != "" {
				id = a.CVE
			}

			severity := "unknown"
			// Composer audit doesn't provide severity directly,
			// but we can infer from source (GitHub advisories have it)
			// Default to "high" for safety
			if id != "" {
				severity = "high"
			}

			advisories = append(advisories, dependency.AdvisoryInfo{
				ID:               id,
				Package:          pkgName,
				Severity:         severity,
				Title:            a.Title,
				AffectedVersions: a.AffectedVersions,
				URL:              a.Link,
			})
		}
	}

	return advisories, nil
}
