package scanner

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

func TestListAvailable_NoUpdates(t *testing.T) {
	s := New(&mockRunner{output: []byte(`{}`)})
	updates, err := s.ListAvailable(context.Background(), "/tmp/project")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(updates) != 0 {
		t.Errorf("expected no updates, got %d", len(updates))
	}
}

func TestListAvailable_EmptyOutput(t *testing.T) {
	s := New(&mockRunner{output: []byte(``)})
	updates, err := s.ListAvailable(context.Background(), "/tmp/project")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(updates) != 0 {
		t.Errorf("expected no updates, got %d", len(updates))
	}
}

func TestListAvailable_PatchUpdate(t *testing.T) {
	json := `{"axios": {"current": "1.6.0", "wanted": "1.6.8", "latest": "1.6.8", "location": "node_modules/axios"}}`
	s := New(&mockRunner{output: []byte(json)})
	updates, err := s.ListAvailable(context.Background(), "/tmp/project")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(updates) != 1 {
		t.Fatalf("expected 1 update, got %d", len(updates))
	}
	if updates[0].Name != "axios" {
		t.Errorf("expected name axios, got %s", updates[0].Name)
	}
	if updates[0].UpdateType != "patch" {
		t.Errorf("expected patch update type, got %s", updates[0].UpdateType)
	}
}

func TestListAvailable_MinorUpdate(t *testing.T) {
	json := `{"express": {"current": "4.17.0", "wanted": "4.17.1", "latest": "4.18.2", "location": "node_modules/express"}}`
	s := New(&mockRunner{output: []byte(json)})
	updates, err := s.ListAvailable(context.Background(), "/tmp/project")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(updates) != 1 {
		t.Fatalf("expected 1 update, got %d", len(updates))
	}
	if updates[0].UpdateType != "minor" {
		t.Errorf("expected minor update type, got %s", updates[0].UpdateType)
	}
}

func TestListAvailable_MajorUpdate(t *testing.T) {
	json := `{"webpack": {"current": "4.46.0", "wanted": "4.46.0", "latest": "5.89.0", "location": "node_modules/webpack"}}`
	s := New(&mockRunner{output: []byte(json)})
	updates, err := s.ListAvailable(context.Background(), "/tmp/project")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(updates) != 1 {
		t.Fatalf("expected 1 update, got %d", len(updates))
	}
	if updates[0].UpdateType != "major" {
		t.Errorf("expected major update type, got %s", updates[0].UpdateType)
	}
}

func TestListAvailable_MixedUpdates(t *testing.T) {
	json := `{
		"axios":   {"current": "1.6.0", "wanted": "1.6.8", "latest": "1.6.8", "location": "node_modules/axios"},
		"express": {"current": "4.17.0", "wanted": "4.17.1", "latest": "4.18.2", "location": "node_modules/express"},
		"webpack": {"current": "4.46.0", "wanted": "4.46.0", "latest": "5.89.0", "location": "node_modules/webpack"}
	}`
	s := New(&mockRunner{output: []byte(json)})
	updates, err := s.ListAvailable(context.Background(), "/tmp/project")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(updates) != 3 {
		t.Fatalf("expected 3 updates, got %d", len(updates))
	}

	types := map[string]string{}
	for _, u := range updates {
		types[u.Name] = u.UpdateType
	}
	if types["axios"] != "patch" {
		t.Errorf("axios: expected patch, got %s", types["axios"])
	}
	if types["express"] != "minor" {
		t.Errorf("express: expected minor, got %s", types["express"])
	}
	if types["webpack"] != "major" {
		t.Errorf("webpack: expected major, got %s", types["webpack"])
	}
}

func TestListAvailable_CurrentEqualsLatest(t *testing.T) {
	json := `{"lodash": {"current": "4.17.21", "wanted": "4.17.21", "latest": "4.17.21", "location": "node_modules/lodash"}}`
	s := New(&mockRunner{output: []byte(json)})
	updates, err := s.ListAvailable(context.Background(), "/tmp/project")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(updates) != 0 {
		t.Errorf("expected no updates, got %d", len(updates))
	}
}

func TestListAvailable_MissingCurrentOrLatest(t *testing.T) {
	json := `{
		"pkg-no-current": {"current": "", "wanted": "1.0.0", "latest": "1.0.0", "location": "node_modules/pkg-no-current"},
		"pkg-no-latest":  {"current": "1.0.0", "wanted": "1.0.0", "latest": "", "location": "node_modules/pkg-no-latest"}
	}`
	s := New(&mockRunner{output: []byte(json)})
	updates, err := s.ListAvailable(context.Background(), "/tmp/project")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(updates) != 0 {
		t.Errorf("expected no updates (skipped gracefully), got %d", len(updates))
	}
}

func TestListAvailable_RunnerError(t *testing.T) {
	s := New(&mockRunner{err: errors.New("exec failed")})
	_, err := s.ListAvailable(context.Background(), "/tmp/project")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestListAvailable_InvalidJSON(t *testing.T) {
	s := New(&mockRunner{output: []byte(`not valid json`)})
	_, err := s.ListAvailable(context.Background(), "/tmp/project")
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}
