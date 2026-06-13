package security

import (
	"context"
	"errors"
	"testing"
)

type mockRunner struct {
	output []byte
	err    error
}

func (m *mockRunner) Run(ctx context.Context, dir string, name string, args ...string) ([]byte, error) {
	return m.output, m.err
}

func TestNpmAuditScanner_CleanAudit(t *testing.T) {
	runner := &mockRunner{
		output: []byte(`{"vulnerabilities": {}}`),
	}
	scanner := NewNpmAuditScanner(runner)

	advisories, err := scanner.Scan(context.Background(), "/tmp/project")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(advisories) != 0 {
		t.Fatalf("expected 0 advisories, got: %d", len(advisories))
	}
}

func TestNpmAuditScanner_SingleVulnerability(t *testing.T) {
	runner := &mockRunner{
		output: []byte(`{
  "vulnerabilities": {
    "lodash": {
      "name": "lodash",
      "severity": "high",
      "via": [
        {
          "source": 1094249,
          "name": "lodash",
          "title": "Prototype Pollution",
          "url": "https://github.com/advisories/GHSA-1234",
          "severity": "high",
          "range": "<4.17.21"
        }
      ],
      "fixAvailable": true,
      "range": "<=4.17.20"
    }
  }
}`),
	}
	scanner := NewNpmAuditScanner(runner)

	advisories, err := scanner.Scan(context.Background(), "/tmp/project")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(advisories) != 1 {
		t.Fatalf("expected 1 advisory, got: %d", len(advisories))
	}

	adv := advisories[0]
	if adv.ID != "1094249" {
		t.Errorf("expected ID '1094249', got: %q", adv.ID)
	}
	if adv.Severity != "high" {
		t.Errorf("expected Severity 'high', got: %q", adv.Severity)
	}
	if adv.Title != "Prototype Pollution" {
		t.Errorf("expected Title 'Prototype Pollution', got: %q", adv.Title)
	}
}

func TestNpmAuditScanner_MultipleVulnerabilities(t *testing.T) {
	runner := &mockRunner{
		output: []byte(`{
  "vulnerabilities": {
    "axios": {
      "name": "axios",
      "severity": "moderate",
      "via": [
        {"source": 1234, "name": "axios", "title": "SSRF", "url": "https://github.com/advisories/GHSA-5678", "severity": "moderate", "range": "<1.6.0"}
      ],
      "fixAvailable": true,
      "range": "<=1.5.0"
    },
    "minimist": {
      "name": "minimist",
      "severity": "critical",
      "via": [
        {"source": 5678, "name": "minimist", "title": "Prototype Pollution", "url": "https://github.com/advisories/GHSA-9012", "severity": "critical", "range": "<1.2.6"}
      ],
      "fixAvailable": true,
      "range": "<1.2.6"
    }
  }
}`),
	}
	scanner := NewNpmAuditScanner(runner)

	advisories, err := scanner.Scan(context.Background(), "/tmp/project")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(advisories) != 2 {
		t.Fatalf("expected 2 advisories, got: %d", len(advisories))
	}
}

func TestNpmAuditScanner_TransitiveDependencyViaString(t *testing.T) {
	runner := &mockRunner{
		output: []byte(`{
  "vulnerabilities": {
    "mkdirp": {
      "name": "mkdirp",
      "severity": "low",
      "via": ["minimist"],
      "fixAvailable": true,
      "range": "0.4.1 - 0.5.1"
    }
  }
}`),
	}
	scanner := NewNpmAuditScanner(runner)

	advisories, err := scanner.Scan(context.Background(), "/tmp/project")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(advisories) != 0 {
		t.Fatalf("expected 0 advisories (transitive skipped), got: %d", len(advisories))
	}
}

func TestNpmAuditScanner_EmptyOutput(t *testing.T) {
	runner := &mockRunner{
		output: []byte{},
	}
	scanner := NewNpmAuditScanner(runner)

	advisories, err := scanner.Scan(context.Background(), "/tmp/project")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if advisories != nil {
		t.Fatalf("expected nil advisories, got: %v", advisories)
	}
}

func TestNpmAuditScanner_RunnerError(t *testing.T) {
	runner := &mockRunner{
		err: errors.New("command failed"),
	}
	scanner := NewNpmAuditScanner(runner)

	_, err := scanner.Scan(context.Background(), "/tmp/project")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestNpmAuditScanner_InvalidJSON(t *testing.T) {
	runner := &mockRunner{
		output: []byte(`not valid json at all {{{`),
	}
	scanner := NewNpmAuditScanner(runner)

	_, err := scanner.Scan(context.Background(), "/tmp/project")
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}
