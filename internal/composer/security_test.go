package composer

import (
	"context"
	"errors"
	"testing"
)

func TestAuditScanner_CleanAudit(t *testing.T) {
	runner := &mockRunner{
		output: []byte(`{"advisories": {}}`),
	}
	scanner := NewAuditScanner(runner, true)

	advisories, err := scanner.Scan(context.Background(), "/tmp/project")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(advisories) != 0 {
		t.Fatalf("expected 0 advisories, got: %d", len(advisories))
	}
}

func TestAuditScanner_SingleVulnerability(t *testing.T) {
	runner := &mockRunner{
		output: []byte(`{
  "advisories": {
    "guzzlehttp/guzzle": [
      {
        "advisoryId": "GHSA-25mq-v84q-4j7r",
        "packageName": "guzzlehttp/guzzle",
        "title": "CURLOPT_HTTPAUTH option not cleared on change of origin",
        "link": "https://github.com/advisories/GHSA-25mq-v84q-4j7r",
        "cve": "CVE-2022-29248",
        "affectedVersions": ">=7.0.0,<7.4.5",
        "sources": [{"name": "GitHub", "remoteId": "GHSA-25mq-v84q-4j7r"}]
      }
    ]
  }
}`),
	}
	scanner := NewAuditScanner(runner, true)

	advisories, err := scanner.Scan(context.Background(), "/tmp/project")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(advisories) != 1 {
		t.Fatalf("expected 1 advisory, got: %d", len(advisories))
	}

	adv := advisories[0]
	if adv.ID != "GHSA-25mq-v84q-4j7r" {
		t.Errorf("expected ID 'GHSA-25mq-v84q-4j7r', got: %q", adv.ID)
	}
	if adv.Package != "guzzlehttp/guzzle" {
		t.Errorf("expected Package 'guzzlehttp/guzzle', got: %q", adv.Package)
	}
	if adv.Title != "CURLOPT_HTTPAUTH option not cleared on change of origin" {
		t.Errorf("expected correct Title, got: %q", adv.Title)
	}
	if adv.URL != "https://github.com/advisories/GHSA-25mq-v84q-4j7r" {
		t.Errorf("expected correct URL, got: %q", adv.URL)
	}
}

func TestAuditScanner_MultipleVulnerabilities(t *testing.T) {
	runner := &mockRunner{
		output: []byte(`{
  "advisories": {
    "guzzlehttp/guzzle": [
      {
        "advisoryId": "GHSA-25mq-v84q-4j7r",
        "packageName": "guzzlehttp/guzzle",
        "title": "CURLOPT_HTTPAUTH option not cleared on change of origin",
        "link": "https://github.com/advisories/GHSA-25mq-v84q-4j7r",
        "cve": "CVE-2022-29248",
        "affectedVersions": ">=7.0.0,<7.4.5",
        "sources": []
      },
      {
        "advisoryId": "GHSA-cwmx-4jbh-v84q",
        "packageName": "guzzlehttp/guzzle",
        "title": "Failure to strip the Cookie header on change in host or HTTP downgrade",
        "link": "https://github.com/advisories/GHSA-cwmx-4jbh-v84q",
        "cve": "CVE-2022-31090",
        "affectedVersions": ">=7.0.0,<7.4.5",
        "sources": []
      }
    ],
    "symfony/http-kernel": [
      {
        "advisoryId": "GHSA-q2x9-j6m3-j5xv",
        "packageName": "symfony/http-kernel",
        "title": "Symfony HTTP Kernel vulnerable to open redirect",
        "link": "https://github.com/advisories/GHSA-q2x9-j6m3-j5xv",
        "cve": "CVE-2022-24894",
        "affectedVersions": ">=2.0.0,<5.4.20",
        "sources": []
      }
    ]
  }
}`),
	}
	scanner := NewAuditScanner(runner, true)

	advisories, err := scanner.Scan(context.Background(), "/tmp/project")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(advisories) != 3 {
		t.Fatalf("expected 3 advisories, got: %d", len(advisories))
	}
}

func TestAuditScanner_EmptyOutput(t *testing.T) {
	runner := &mockRunner{
		output: []byte{},
	}
	scanner := NewAuditScanner(runner, true)

	advisories, err := scanner.Scan(context.Background(), "/tmp/project")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if advisories != nil {
		t.Fatalf("expected nil advisories, got: %v", advisories)
	}
}

func TestAuditScanner_RunnerError(t *testing.T) {
	runner := &mockRunner{
		err: errors.New("command failed"),
	}
	scanner := NewAuditScanner(runner, true)

	_, err := scanner.Scan(context.Background(), "/tmp/project")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestAuditScanner_InvalidJSON(t *testing.T) {
	runner := &mockRunner{
		output: []byte(`not valid json at all {{{`),
	}
	scanner := NewAuditScanner(runner, true)

	_, err := scanner.Scan(context.Background(), "/tmp/project")
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

func TestAuditScanner_CVEFallbackWhenNoAdvisoryID(t *testing.T) {
	runner := &mockRunner{
		output: []byte(`{
  "advisories": {
    "some/package": [
      {
        "advisoryId": "",
        "packageName": "some/package",
        "title": "Some vulnerability",
        "link": "https://example.com/advisory",
        "cve": "CVE-2023-12345",
        "affectedVersions": ">=1.0.0,<1.2.0",
        "sources": []
      }
    ]
  }
}`),
	}
	scanner := NewAuditScanner(runner, true)

	advisories, err := scanner.Scan(context.Background(), "/tmp/project")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(advisories) != 1 {
		t.Fatalf("expected 1 advisory, got: %d", len(advisories))
	}

	adv := advisories[0]
	if adv.ID != "CVE-2023-12345" {
		t.Errorf("expected ID to fall back to CVE 'CVE-2023-12345', got: %q", adv.ID)
	}
}
