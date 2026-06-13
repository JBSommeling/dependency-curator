package dependency

import (
	"testing"

	"github.com/JBSommeling/dependency-curator/internal/scanner"
	"github.com/JBSommeling/dependency-curator/internal/security"
)

func TestEnrich_UpdateOnly(t *testing.T) {
	deps := []Dependency{
		{Name: "lodash", CurrentVersion: "4.0.0"},
	}
	updates := []scanner.Update{
		{Name: "lodash", Current: "4.0.0", Latest: "4.17.21", UpdateType: "patch"},
	}

	result := Enrich(deps, updates, nil)

	if len(result) != 1 {
		t.Fatalf("expected 1 dep, got %d", len(result))
	}
	if result[0].LatestVersion != "4.17.21" {
		t.Errorf("LatestVersion: got %q, want %q", result[0].LatestVersion, "4.17.21")
	}
	if result[0].UpdateType != "patch" {
		t.Errorf("UpdateType: got %q, want %q", result[0].UpdateType, "patch")
	}
	if len(result[0].Advisories) != 0 {
		t.Errorf("expected no advisories, got %d", len(result[0].Advisories))
	}
}

func TestEnrich_AdvisoryOnly(t *testing.T) {
	deps := []Dependency{
		{Name: "lodash", CurrentVersion: "4.0.0", UpdateType: "none"},
	}
	advisories := []security.Advisory{
		{ID: "1234", Package: "lodash", Severity: "high", Title: "Prototype Pollution", AffectedVersions: "<4.17.21", FixedVersion: "4.17.21", URL: "https://example.com/1234"},
	}

	result := Enrich(deps, nil, advisories)

	if len(result) != 1 {
		t.Fatalf("expected 1 dep, got %d", len(result))
	}
	if result[0].UpdateType != "none" {
		t.Errorf("UpdateType should be unchanged: got %q, want %q", result[0].UpdateType, "none")
	}
	if len(result[0].Advisories) != 1 {
		t.Fatalf("expected 1 advisory, got %d", len(result[0].Advisories))
	}
	a := result[0].Advisories[0]
	if a.ID != "1234" || a.Severity != "high" || a.Title != "Prototype Pollution" {
		t.Errorf("advisory fields mismatch: %+v", a)
	}
}

func TestEnrich_BothUpdateAndAdvisory(t *testing.T) {
	deps := []Dependency{
		{Name: "express", CurrentVersion: "4.0.0"},
	}
	updates := []scanner.Update{
		{Name: "express", Current: "4.0.0", Latest: "4.18.0", UpdateType: "minor"},
	}
	advisories := []security.Advisory{
		{ID: "5678", Package: "express", Severity: "moderate", Title: "Open Redirect", AffectedVersions: "<4.18.0", FixedVersion: "4.18.0", URL: "https://example.com/5678"},
	}

	result := Enrich(deps, updates, advisories)

	if len(result) != 1 {
		t.Fatalf("expected 1 dep, got %d", len(result))
	}
	if result[0].LatestVersion != "4.18.0" {
		t.Errorf("LatestVersion: got %q, want %q", result[0].LatestVersion, "4.18.0")
	}
	if result[0].UpdateType != "minor" {
		t.Errorf("UpdateType: got %q, want %q", result[0].UpdateType, "minor")
	}
	if len(result[0].Advisories) != 1 {
		t.Fatalf("expected 1 advisory, got %d", len(result[0].Advisories))
	}
}

func TestEnrich_Neither(t *testing.T) {
	deps := []Dependency{
		{Name: "react", CurrentVersion: "18.0.0", LatestVersion: "18.0.0", UpdateType: "none"},
	}

	result := Enrich(deps, nil, nil)

	if len(result) != 1 {
		t.Fatalf("expected 1 dep, got %d", len(result))
	}
	if result[0].Name != "react" || result[0].CurrentVersion != "18.0.0" || result[0].LatestVersion != "18.0.0" || result[0].UpdateType != "none" {
		t.Errorf("dep should be unchanged: %+v", result[0])
	}
	if len(result[0].Advisories) != 0 {
		t.Errorf("expected no advisories, got %d", len(result[0].Advisories))
	}
}

func TestEnrich_MultipleAdvisories(t *testing.T) {
	deps := []Dependency{
		{Name: "moment", CurrentVersion: "2.0.0"},
	}
	advisories := []security.Advisory{
		{ID: "A1", Package: "moment", Severity: "high", Title: "ReDoS"},
		{ID: "A2", Package: "moment", Severity: "moderate", Title: "Path Traversal"},
	}

	result := Enrich(deps, nil, advisories)

	if len(result) != 1 {
		t.Fatalf("expected 1 dep, got %d", len(result))
	}
	if len(result[0].Advisories) != 2 {
		t.Fatalf("expected 2 advisories, got %d", len(result[0].Advisories))
	}
}

func TestEnrich_UpdateForUnknownDep(t *testing.T) {
	deps := []Dependency{
		{Name: "react", CurrentVersion: "18.0.0"},
	}
	updates := []scanner.Update{
		{Name: "lodash", Current: "4.0.0", Latest: "4.17.21", UpdateType: "patch"},
	}

	result := Enrich(deps, updates, nil)

	if len(result) != 1 {
		t.Fatalf("expected 1 dep, got %d", len(result))
	}
	// react should be unchanged; the lodash update should be ignored
	if result[0].LatestVersion != "" {
		t.Errorf("LatestVersion should be empty, got %q", result[0].LatestVersion)
	}
	if result[0].UpdateType != "" {
		t.Errorf("UpdateType should be empty, got %q", result[0].UpdateType)
	}
}

func TestEnrich_AdvisoryForUnknownDep(t *testing.T) {
	deps := []Dependency{
		{Name: "react", CurrentVersion: "18.0.0"},
	}
	advisories := []security.Advisory{
		{ID: "X1", Package: "lodash", Severity: "critical", Title: "Critical Bug"},
	}

	result := Enrich(deps, nil, advisories)

	if len(result) != 1 {
		t.Fatalf("expected 1 dep, got %d", len(result))
	}
	if len(result[0].Advisories) != 0 {
		t.Errorf("expected no advisories on react, got %d", len(result[0].Advisories))
	}
}

func TestEnrich_EmptyInputs(t *testing.T) {
	result := Enrich(nil, nil, nil)

	if len(result) != 0 {
		t.Errorf("expected empty result, got %d items", len(result))
	}
}

func TestEnrich_PreservesOtherFields(t *testing.T) {
	deps := []Dependency{
		{Name: "webpack", CurrentVersion: "5.0.0", IsDev: true, UpdateType: "none"},
	}
	updates := []scanner.Update{
		{Name: "webpack", Current: "5.0.0", Latest: "5.90.0", UpdateType: "minor"},
	}

	result := Enrich(deps, updates, nil)

	if len(result) != 1 {
		t.Fatalf("expected 1 dep, got %d", len(result))
	}
	d := result[0]
	if d.Name != "webpack" {
		t.Errorf("Name: got %q, want %q", d.Name, "webpack")
	}
	if d.CurrentVersion != "5.0.0" {
		t.Errorf("CurrentVersion: got %q, want %q", d.CurrentVersion, "5.0.0")
	}
	if !d.IsDev {
		t.Errorf("IsDev should be true")
	}
	if d.LatestVersion != "5.90.0" {
		t.Errorf("LatestVersion: got %q, want %q", d.LatestVersion, "5.90.0")
	}
	if d.UpdateType != "minor" {
		t.Errorf("UpdateType: got %q, want %q", d.UpdateType, "minor")
	}
}
