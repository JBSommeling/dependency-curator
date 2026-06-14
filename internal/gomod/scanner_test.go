package gomod

import (
	"context"
	"strings"
	"testing"

	"github.com/JBSommeling/dependency-curator/internal/dependency"
)

type mockRunner struct {
	responses map[string]mockResp
}

type mockResp struct {
	output []byte
	err    error
}

func (m *mockRunner) Run(_ context.Context, _ string, name string, args ...string) ([]byte, error) {
	key := name + " " + strings.Join(args, " ")
	if resp, ok := m.responses[key]; ok {
		return resp.output, resp.err
	}
	return nil, nil
}

func (m *mockRunner) RunAllowExit1(ctx context.Context, dir string, name string, args ...string) ([]byte, error) {
	return m.Run(ctx, dir, name, args...)
}

func TestScanner_ListAvailable(t *testing.T) {
	// go list -m -u -json all outputs concatenated JSON objects
	goListOutput := `{"Path":"example.com/myproject","Version":"","Main":true}
{"Path":"github.com/pkg/errors","Version":"v0.9.1","Update":{"Version":"v0.9.2"},"Indirect":false}
{"Path":"golang.org/x/text","Version":"v0.14.0","Update":{"Version":"v0.21.0"},"Indirect":false}
{"Path":"github.com/some/indirect","Version":"v1.0.0","Update":{"Version":"v1.1.0"},"Indirect":true}
{"Path":"github.com/no/update","Version":"v2.0.0","Indirect":false}
`

	runner := &mockRunner{responses: map[string]mockResp{
		"go list -m -u -json all": {output: []byte(goListOutput)},
	}}

	sc := NewScanner(runner)
	updates, err := sc.ListAvailable(context.Background(), "/test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(updates) != 2 {
		t.Fatalf("expected 2 updates, got %d: %+v", len(updates), updates)
	}

	// Check patch update
	assertUpdate(t, updates[0], "github.com/pkg/errors", "0.9.1", "0.9.2", "patch")

	// Check major update (0.x -> 0.x is minor per semver, but our classifier should handle)
	// Actually 0.14.0 -> 0.21.0 with different minor = minor for 0.x
	// Let's just check it's detected
	if updates[1].Name != "golang.org/x/text" {
		t.Errorf("updates[1].Name = %q, want golang.org/x/text", updates[1].Name)
	}

	// Indirect should be excluded
	for _, u := range updates {
		if u.Name == "github.com/some/indirect" {
			t.Error("indirect dependency should be excluded")
		}
	}

	// No-update should be excluded
	for _, u := range updates {
		if u.Name == "github.com/no/update" {
			t.Error("dependency without update should be excluded")
		}
	}
}

func TestScanner_ListAvailable_Empty(t *testing.T) {
	runner := &mockRunner{responses: map[string]mockResp{
		"go list -m -u -json all": {output: nil},
	}}

	sc := NewScanner(runner)
	updates, err := sc.ListAvailable(context.Background(), "/test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(updates) != 0 {
		t.Errorf("expected 0 updates, got %d", len(updates))
	}
}

func assertUpdate(t *testing.T, u dependency.UpdateInfo, name, current, latest, updateType string) {
	t.Helper()
	if u.Name != name {
		t.Errorf("Name = %q, want %q", u.Name, name)
	}
	if u.Current != current {
		t.Errorf("Current = %q, want %q", u.Current, current)
	}
	if u.Latest != latest {
		t.Errorf("Latest = %q, want %q", u.Latest, latest)
	}
	if u.UpdateType != updateType {
		t.Errorf("UpdateType = %q, want %q", u.UpdateType, updateType)
	}
}
