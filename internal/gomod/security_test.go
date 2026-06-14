package gomod

import (
	"context"
	"errors"
	"testing"
)

func TestVulnScanner_Scan(t *testing.T) {
	govulnOutput := `{"osv":{"id":"GO-2024-0001","summary":"SQL injection in database/sql","affected":[{"package":{"name":"github.com/vulnerable/pkg"},"ranges":[{"events":[{"introduced":"0"},{"fixed":"1.2.3"}]}]}]}}
{"finding":{"osv":"GO-2024-0001","trace":[{"module":"github.com/vulnerable/pkg","version":"v1.1.0"}]}}
`

	runner := &mockRunner{responses: map[string]mockResp{
		"govulncheck -json ./...": {output: []byte(govulnOutput)},
	}}

	sc := NewVulnScanner(runner)
	advisories, err := sc.Scan(context.Background(), "/test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(advisories) != 1 {
		t.Fatalf("expected 1 advisory, got %d: %+v", len(advisories), advisories)
	}

	adv := advisories[0]
	if adv.ID != "GO-2024-0001" {
		t.Errorf("ID = %q, want GO-2024-0001", adv.ID)
	}
	if adv.Package != "github.com/vulnerable/pkg" {
		t.Errorf("Package = %q, want github.com/vulnerable/pkg", adv.Package)
	}
	if adv.Title != "SQL injection in database/sql" {
		t.Errorf("Title = %q, want SQL injection in database/sql", adv.Title)
	}
	if adv.FixedVersion != "1.2.3" {
		t.Errorf("FixedVersion = %q, want 1.2.3", adv.FixedVersion)
	}
	if adv.URL != "https://pkg.go.dev/vuln/GO-2024-0001" {
		t.Errorf("URL = %q, want https://pkg.go.dev/vuln/GO-2024-0001", adv.URL)
	}
}

func TestVulnScanner_Scan_NotInstalled(t *testing.T) {
	runner := &mockRunner{responses: map[string]mockResp{
		"govulncheck -json ./...": {err: errors.New("exec: \"govulncheck\": executable file not found in $PATH")},
	}}

	sc := NewVulnScanner(runner)
	advisories, err := sc.Scan(context.Background(), "/test")
	if err != nil {
		t.Fatalf("should not error when govulncheck not found: %v", err)
	}
	if len(advisories) != 0 {
		t.Errorf("expected 0 advisories, got %d", len(advisories))
	}
}

func TestVulnScanner_Scan_Empty(t *testing.T) {
	runner := &mockRunner{responses: map[string]mockResp{
		"govulncheck -json ./...": {output: nil},
	}}

	sc := NewVulnScanner(runner)
	advisories, err := sc.Scan(context.Background(), "/test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(advisories) != 0 {
		t.Errorf("expected 0 advisories, got %d", len(advisories))
	}
}

func TestIsNotFound(t *testing.T) {
	cases := []struct {
		err  error
		want bool
	}{
		{errors.New("exec: \"govulncheck\": executable file not found in $PATH"), true},
		{errors.New("govulncheck: command not found"), true},
		{errors.New("no such file or directory"), true},
		{errors.New("exit status 1"), false},
		{errors.New("permission denied"), false},
	}
	for _, tc := range cases {
		got := isNotFound(tc.err)
		if got != tc.want {
			t.Errorf("isNotFound(%q) = %v, want %v", tc.err, got, tc.want)
		}
	}
}
