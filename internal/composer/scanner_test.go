package composer

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

func (m *mockRunner) RunAllowExit1(ctx context.Context, dir string, name string, args ...string) ([]byte, error) {
	return m.output, m.err
}

func TestListAvailable_NoUpdates(t *testing.T) {
	runner := &mockRunner{
		output: []byte(`{"installed": []}`),
	}
	scanner := NewScanner(runner, true)

	updates, err := scanner.ListAvailable(context.Background(), "/tmp/project")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(updates) != 0 {
		t.Fatalf("expected 0 updates, got: %d", len(updates))
	}
}

func TestListAvailable_EmptyOutput(t *testing.T) {
	runner := &mockRunner{
		output: []byte{},
	}
	scanner := NewScanner(runner, true)

	updates, err := scanner.ListAvailable(context.Background(), "/tmp/project")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if updates != nil {
		t.Fatalf("expected nil updates, got: %v", updates)
	}
}

func TestListAvailable_PatchUpdate(t *testing.T) {
	runner := &mockRunner{
		output: []byte(`{
  "installed": [
    {"name": "guzzlehttp/guzzle", "version": "7.5.0", "latest": "7.5.3", "latest-status": "semver-safe-update"}
  ]
}`),
	}
	scanner := NewScanner(runner, true)

	updates, err := scanner.ListAvailable(context.Background(), "/tmp/project")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(updates) != 1 {
		t.Fatalf("expected 1 update, got: %d", len(updates))
	}

	u := updates[0]
	if u.Name != "guzzlehttp/guzzle" {
		t.Errorf("expected name 'guzzlehttp/guzzle', got: %q", u.Name)
	}
	if u.UpdateType != "patch" {
		t.Errorf("expected UpdateType 'patch', got: %q", u.UpdateType)
	}
	if u.Current != "7.5.0" {
		t.Errorf("expected Current '7.5.0', got: %q", u.Current)
	}
	if u.Latest != "7.5.3" {
		t.Errorf("expected Latest '7.5.3', got: %q", u.Latest)
	}
}

func TestListAvailable_MinorUpdate(t *testing.T) {
	runner := &mockRunner{
		output: []byte(`{
  "installed": [
    {"name": "laravel/framework", "version": "10.0.0", "latest": "10.48.0", "latest-status": "semver-safe-update"}
  ]
}`),
	}
	scanner := NewScanner(runner, true)

	updates, err := scanner.ListAvailable(context.Background(), "/tmp/project")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(updates) != 1 {
		t.Fatalf("expected 1 update, got: %d", len(updates))
	}

	if updates[0].UpdateType != "minor" {
		t.Errorf("expected UpdateType 'minor', got: %q", updates[0].UpdateType)
	}
}

func TestListAvailable_MajorUpdate(t *testing.T) {
	runner := &mockRunner{
		output: []byte(`{
  "installed": [
    {"name": "phpunit/phpunit", "version": "9.6.0", "latest": "10.5.0", "latest-status": "update-possible"}
  ]
}`),
	}
	scanner := NewScanner(runner, true)

	updates, err := scanner.ListAvailable(context.Background(), "/tmp/project")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(updates) != 1 {
		t.Fatalf("expected 1 update, got: %d", len(updates))
	}

	if updates[0].UpdateType != "major" {
		t.Errorf("expected UpdateType 'major', got: %q", updates[0].UpdateType)
	}
}

func TestListAvailable_MixedUpdates(t *testing.T) {
	runner := &mockRunner{
		output: []byte(`{
  "installed": [
    {"name": "guzzlehttp/guzzle", "version": "7.5.0", "latest": "7.5.3", "latest-status": "semver-safe-update"},
    {"name": "laravel/framework", "version": "10.0.0", "latest": "10.48.0", "latest-status": "semver-safe-update"},
    {"name": "phpunit/phpunit", "version": "9.6.0", "latest": "10.5.0", "latest-status": "update-possible"}
  ]
}`),
	}
	scanner := NewScanner(runner, true)

	updates, err := scanner.ListAvailable(context.Background(), "/tmp/project")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(updates) != 3 {
		t.Fatalf("expected 3 updates, got: %d", len(updates))
	}

	byName := make(map[string]string)
	for _, u := range updates {
		byName[u.Name] = u.UpdateType
	}

	if byName["guzzlehttp/guzzle"] != "patch" {
		t.Errorf("expected guzzlehttp/guzzle to be 'patch', got: %q", byName["guzzlehttp/guzzle"])
	}
	if byName["laravel/framework"] != "minor" {
		t.Errorf("expected laravel/framework to be 'minor', got: %q", byName["laravel/framework"])
	}
	if byName["phpunit/phpunit"] != "major" {
		t.Errorf("expected phpunit/phpunit to be 'major', got: %q", byName["phpunit/phpunit"])
	}
}

func TestListAvailable_CurrentEqualsLatest(t *testing.T) {
	runner := &mockRunner{
		output: []byte(`{
  "installed": [
    {"name": "guzzlehttp/guzzle", "version": "7.5.0", "latest": "7.5.0", "latest-status": "up-to-date"}
  ]
}`),
	}
	scanner := NewScanner(runner, true)

	updates, err := scanner.ListAvailable(context.Background(), "/tmp/project")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(updates) != 0 {
		t.Fatalf("expected 0 updates (same version skipped), got: %d", len(updates))
	}
}

func TestListAvailable_RunnerError(t *testing.T) {
	runner := &mockRunner{
		err: errors.New("command failed"),
	}
	scanner := NewScanner(runner, true)

	_, err := scanner.ListAvailable(context.Background(), "/tmp/project")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestListAvailable_InvalidJSON(t *testing.T) {
	runner := &mockRunner{
		output: []byte(`not valid json at all {{{`),
	}
	scanner := NewScanner(runner, true)

	_, err := scanner.ListAvailable(context.Background(), "/tmp/project")
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

func TestListAvailable_RawArrayFormat(t *testing.T) {
	// Some Composer versions return a raw array instead of {"installed": [...]}
	rawArray := `[
		{"name": "monolog/monolog", "version": "2.0.0", "latest": "2.0.1", "latest-status": "semver-safe-update"},
		{"name": "symfony/console", "version": "5.0.0", "latest": "6.0.0", "latest-status": "update-possible"}
	]`

	runner := &mockRunner{
		output: []byte(rawArray),
	}

	s := NewScanner(runner, true)
	updates, err := s.ListAvailable(context.Background(), "/test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(updates) != 2 {
		t.Fatalf("expected 2 updates, got %d", len(updates))
	}

	if updates[0].Name != "monolog/monolog" {
		t.Errorf("updates[0].Name = %q, want monolog/monolog", updates[0].Name)
	}
	if updates[0].UpdateType != "patch" {
		t.Errorf("updates[0].UpdateType = %q, want patch", updates[0].UpdateType)
	}
	if updates[1].Name != "symfony/console" {
		t.Errorf("updates[1].Name = %q, want symfony/console", updates[1].Name)
	}
}

func TestListAvailable_VersionWithVPrefix(t *testing.T) {
	runner := &mockRunner{
		output: []byte(`{
  "installed": [
    {"name": "guzzlehttp/guzzle", "version": "v7.5.0", "latest": "v7.5.3", "latest-status": "semver-safe-update"}
  ]
}`),
	}
	scanner := NewScanner(runner, true)

	updates, err := scanner.ListAvailable(context.Background(), "/tmp/project")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(updates) != 1 {
		t.Fatalf("expected 1 update, got: %d", len(updates))
	}

	u := updates[0]
	if u.UpdateType != "patch" {
		t.Errorf("expected UpdateType 'patch', got: %q", u.UpdateType)
	}
	if u.Current != "7.5.0" {
		t.Errorf("expected Current '7.5.0' (v prefix stripped), got: %q", u.Current)
	}
	if u.Latest != "7.5.3" {
		t.Errorf("expected Latest '7.5.3' (v prefix stripped), got: %q", u.Latest)
	}
}
