package reporting

import (
	"strings"
	"testing"

	"github.com/JBSommeling/dependency-curator/internal/changelog"
	"github.com/JBSommeling/dependency-curator/internal/dependency"
)

func TestGeneratePRBody_Empty(t *testing.T) {
	body := GeneratePRBody([]dependency.Dependency{}, nil)

	if !strings.Contains(body, "# Dependency Guardian Report") {
		t.Error("expected report header")
	}
	if !strings.Contains(body, "## Executive Summary") {
		t.Error("expected executive summary section")
	}
	if !strings.Contains(body, "| Total updates | 0 |") {
		t.Error("expected zero total updates")
	}
	if !strings.Contains(body, "## Validation Checklist") {
		t.Error("expected validation checklist")
	}
	if !strings.Contains(body, "## Risk Assessment") {
		t.Error("expected risk assessment")
	}
}

func TestGeneratePRBody_PatchesOnly(t *testing.T) {
	deps := []dependency.Dependency{
		{Name: "pkg-a", CurrentVersion: "1.0.0", LatestVersion: "1.0.1", UpdateType: "patch"},
		{Name: "pkg-b", CurrentVersion: "2.3.0", LatestVersion: "2.3.4", UpdateType: "patch"},
	}
	body := GeneratePRBody(deps, nil)

	if !strings.Contains(body, "| Patch updates | 2 |") {
		t.Error("expected 2 patch updates in summary")
	}
	if !strings.Contains(body, "| Total updates | 2 |") {
		t.Error("expected total of 2")
	}
	if strings.Contains(body, "## Breaking Change Analysis") {
		t.Error("did not expect breaking change analysis section for patches only")
	}
	if strings.Contains(body, "## Security Impact") {
		t.Error("did not expect security impact section for no advisories")
	}
	if !strings.Contains(body, "## Updated Dependencies") {
		t.Error("expected updated dependencies section")
	}
	if !strings.Contains(body, "pkg-a") {
		t.Error("expected pkg-a in updated dependencies")
	}
	if !strings.Contains(body, "pkg-b") {
		t.Error("expected pkg-b in updated dependencies")
	}
}

func TestGeneratePRBody_MixedUpdates(t *testing.T) {
	deps := []dependency.Dependency{
		{Name: "pkg-patch", CurrentVersion: "1.0.0", LatestVersion: "1.0.1", UpdateType: "patch"},
		{Name: "pkg-minor", CurrentVersion: "2.0.0", LatestVersion: "2.1.0", UpdateType: "minor"},
		{Name: "pkg-major", CurrentVersion: "3.0.0", LatestVersion: "4.0.0", UpdateType: "major"},
	}
	body := GeneratePRBody(deps, nil)

	if !strings.Contains(body, "| Patch updates | 1 |") {
		t.Error("expected 1 patch update")
	}
	if !strings.Contains(body, "| Minor updates | 1 |") {
		t.Error("expected 1 minor update")
	}
	if !strings.Contains(body, "| Major updates | 1 |") {
		t.Error("expected 1 major update")
	}
	if !strings.Contains(body, "| Total updates | 3 |") {
		t.Error("expected total of 3")
	}
	if !strings.Contains(body, "## Updated Dependencies") {
		t.Error("expected updated dependencies section")
	}
	if !strings.Contains(body, "## Breaking Change Analysis") {
		t.Error("expected breaking change analysis section")
	}
	if !strings.Contains(body, "### pkg-major") {
		t.Error("expected major package in breaking changes")
	}
}

func TestGeneratePRBody_WithAdvisories(t *testing.T) {
	deps := []dependency.Dependency{
		{
			Name:          "vulnerable-pkg",
			CurrentVersion: "1.0.0",
			LatestVersion:  "1.0.1",
			UpdateType:    "patch",
			Advisories: []dependency.Advisory{
				{
					ID:       "CVE-2024-1234",
					Severity: "high",
					Title:    "Remote code execution",
					URL:      "https://example.com/cve-2024-1234",
				},
				{
					ID:       "CVE-2024-5678",
					Severity: "low",
					Title:    "Information disclosure",
				},
			},
		},
	}
	body := GeneratePRBody(deps, nil)

	if !strings.Contains(body, "## Security Impact") {
		t.Error("expected security impact section")
	}
	if !strings.Contains(body, "| Package | Severity | Advisory | Title |") {
		t.Error("expected security table header")
	}
	if !strings.Contains(body, "vulnerable-pkg") {
		t.Error("expected package name in security table")
	}
	if !strings.Contains(body, "CVE-2024-1234") {
		t.Error("expected CVE-2024-1234 in security table")
	}
	if !strings.Contains(body, "https://example.com/cve-2024-1234") {
		t.Error("expected URL as link for advisory")
	}
	if !strings.Contains(body, "CVE-2024-5678") {
		t.Error("expected CVE-2024-5678 in security table")
	}
	if !strings.Contains(body, "| Vulnerabilities fixed | 2 |") {
		t.Error("expected 2 vulnerabilities in summary")
	}
	// Verify high severity appears before low (sorted)
	highIdx := strings.Index(body, "CVE-2024-1234")
	lowIdx := strings.Index(body, "CVE-2024-5678")
	if highIdx > lowIdx {
		t.Error("expected high severity advisory to appear before low severity")
	}
}

func TestGeneratePRBody_WithChangelogs(t *testing.T) {
	deps := []dependency.Dependency{
		{Name: "big-pkg", CurrentVersion: "1.0.0", LatestVersion: "2.0.0", UpdateType: "major"},
	}
	changelogs := map[string]*changelog.ChangelogInfo{
		"big-pkg": {
			PackageName:     "big-pkg",
			FromVersion:     "1.0.0",
			ToVersion:       "2.0.0",
			BreakingChanges: []string{"API endpoint /old removed", "Config format changed"},
			MigrationNotes:  []string{"Run migration script", "Update config file"},
			ReleaseNotesURL: "https://example.com/releases/v2",
			ChangelogURL:    "https://example.com/changelog",
			Available:       true,
		},
	}
	body := GeneratePRBody(deps, changelogs)

	if !strings.Contains(body, "## Breaking Change Analysis") {
		t.Error("expected breaking change analysis section")
	}
	if !strings.Contains(body, "**Potential Breaking Changes:**") {
		t.Error("expected breaking changes listed")
	}
	if !strings.Contains(body, "API endpoint /old removed") {
		t.Error("expected first breaking change")
	}
	if !strings.Contains(body, "Config format changed") {
		t.Error("expected second breaking change")
	}
	if !strings.Contains(body, "**Migration Notes:**") {
		t.Error("expected migration notes")
	}
	if !strings.Contains(body, "Run migration script") {
		t.Error("expected migration note")
	}
	if !strings.Contains(body, "**References:**") {
		t.Error("expected references section")
	}
	if !strings.Contains(body, "[Release Notes](https://example.com/releases/v2)") {
		t.Error("expected release notes link")
	}
	if !strings.Contains(body, "[Changelog](https://example.com/changelog)") {
		t.Error("expected changelog link")
	}
}

func TestGeneratePRBody_MajorWithoutChangelog(t *testing.T) {
	deps := []dependency.Dependency{
		{Name: "mystery-pkg", CurrentVersion: "1.0.0", LatestVersion: "2.0.0", UpdateType: "major"},
	}
	body := GeneratePRBody(deps, nil)

	if !strings.Contains(body, "## Breaking Change Analysis") {
		t.Error("expected breaking change analysis section")
	}
	if !strings.Contains(body, "### mystery-pkg") {
		t.Error("expected package heading in breaking changes")
	}
	if !strings.Contains(body, "_Changelog information not available._") {
		t.Error("expected not available message")
	}
}

func TestGenerateSummary_PatchesApplied(t *testing.T) {
	deps := []dependency.Dependency{
		{Name: "pkg-a", UpdateType: "patch"},
		{Name: "pkg-b", UpdateType: "patch"},
	}
	summary := GenerateSummary(deps, 2)

	if !strings.Contains(summary, "## Dependency Guardian Summary") {
		t.Error("expected summary header")
	}
	if !strings.Contains(summary, "**2** patch updates applied") {
		t.Error("expected patch count in summary")
	}
}

func TestGenerateSummary_AllUpToDate(t *testing.T) {
	summary := GenerateSummary([]dependency.Dependency{}, 0)

	if !strings.Contains(summary, "All dependencies are up to date.") {
		t.Error("expected up to date message")
	}
}

func TestGenerateSummary_Mixed(t *testing.T) {
	deps := []dependency.Dependency{
		{Name: "pkg-minor", UpdateType: "minor"},
		{
			Name:       "pkg-vuln",
			UpdateType: "patch",
			Advisories: []dependency.Advisory{
				{ID: "CVE-2024-9999", Severity: "high", Title: "Test vuln"},
			},
		},
	}
	summary := GenerateSummary(deps, 3)

	if !strings.Contains(summary, "**3** patch updates applied") {
		t.Error("expected patch count")
	}
	if !strings.Contains(summary, "**1** minor updates available") {
		t.Error("expected minor count")
	}
	if !strings.Contains(summary, "**1** vulnerabilities found") {
		t.Error("expected vuln count")
	}
}

func TestAssessRisk_High(t *testing.T) {
	r := &Report{
		MajorCount: 1,
		VulnCount:  2,
	}
	risk, reason := assessRisk(r)
	if risk != "High" {
		t.Errorf("expected High risk, got %s", risk)
	}
	if !strings.Contains(reason, "vulnerabilities") {
		t.Error("expected reason to mention vulnerabilities")
	}
}

func TestAssessRisk_Medium_Major(t *testing.T) {
	r := &Report{
		MajorCount: 2,
		VulnCount:  0,
	}
	risk, reason := assessRisk(r)
	if risk != "Medium" {
		t.Errorf("expected Medium risk, got %s", risk)
	}
	if !strings.Contains(reason, "breaking changes") {
		t.Error("expected reason to mention breaking changes")
	}
}

func TestAssessRisk_Medium_Vulns(t *testing.T) {
	r := &Report{
		MinorCount: 1,
		VulnCount:  1,
	}
	risk, reason := assessRisk(r)
	if risk != "Medium" {
		t.Errorf("expected Medium risk, got %s", risk)
	}
	if !strings.Contains(reason, "vulnerabilities") {
		t.Error("expected reason to mention vulnerabilities")
	}
}

func TestAssessRisk_Low(t *testing.T) {
	r := &Report{
		PatchCount: 3,
		VulnCount:  0,
	}
	risk, reason := assessRisk(r)
	if risk != "Low" {
		t.Errorf("expected Low risk, got %s", risk)
	}
	if !strings.Contains(reason, "patch") {
		t.Error("expected reason to mention patch")
	}
}
