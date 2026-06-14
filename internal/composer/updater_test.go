package composer

import (
	"context"
	"errors"
	"testing"

	"github.com/JBSommeling/dependency-curator/internal/dependency"
)

type runCall struct {
	dir  string
	name string
	args []string
}

type callTrackingRunner struct {
	calls []runCall
	errs  map[int]error
}

func (m *callTrackingRunner) Run(ctx context.Context, dir string, name string, args ...string) ([]byte, error) {
	idx := len(m.calls)
	m.calls = append(m.calls, runCall{dir: dir, name: name, args: args})
	if err, ok := m.errs[idx]; ok {
		return nil, err
	}
	return nil, nil
}

func (m *callTrackingRunner) RunAllowExit1(ctx context.Context, dir string, name string, args ...string) ([]byte, error) {
	return m.Run(ctx, dir, name, args...)
}

func TestApplyPatches_NoPatches(t *testing.T) {
	runner := &callTrackingRunner{}
	updater := NewUpdater(runner)

	deps := []dependency.Dependency{
		{Name: "vendor/foo", CurrentVersion: "1.0.0", LatestVersion: "2.0.0", UpdateType: "major"},
		{Name: "vendor/bar", CurrentVersion: "1.0.0", LatestVersion: "1.1.0", UpdateType: "minor"},
	}

	applied, err := updater.ApplyPatches(context.Background(), "/project", deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if applied != nil {
		t.Errorf("expected nil applied, got %v", applied)
	}
	if len(runner.calls) != 0 {
		t.Errorf("expected no runner calls, got %d", len(runner.calls))
	}
}

func TestApplyPatches_SinglePatch(t *testing.T) {
	runner := &callTrackingRunner{}
	updater := NewUpdater(runner)

	deps := []dependency.Dependency{
		{Name: "vendor/foo", CurrentVersion: "1.0.0", LatestVersion: "1.0.1", UpdateType: "patch"},
	}

	applied, err := updater.ApplyPatches(context.Background(), "/project", deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(applied) != 1 {
		t.Fatalf("expected 1 applied, got %d", len(applied))
	}
	if len(runner.calls) != 1 {
		t.Fatalf("expected 1 runner call, got %d", len(runner.calls))
	}

	call := runner.calls[0]
	if call.dir != "/project" {
		t.Errorf("expected dir /project, got %s", call.dir)
	}
	if call.name != "composer" {
		t.Errorf("expected name composer, got %s", call.name)
	}
	wantArgs := []string{"update", "--with", "vendor/foo:1.0.1"}
	if len(call.args) != len(wantArgs) {
		t.Fatalf("expected args %v, got %v", wantArgs, call.args)
	}
	for i, a := range wantArgs {
		if call.args[i] != a {
			t.Errorf("arg[%d]: expected %q, got %q", i, a, call.args[i])
		}
	}
}

func TestApplyPatches_MultiplePatches(t *testing.T) {
	runner := &callTrackingRunner{}
	updater := NewUpdater(runner)

	deps := []dependency.Dependency{
		{Name: "vendor/a", CurrentVersion: "1.0.0", LatestVersion: "1.0.1", UpdateType: "patch"},
		{Name: "vendor/b", CurrentVersion: "2.0.0", LatestVersion: "2.0.3", UpdateType: "patch"},
		{Name: "vendor/c", CurrentVersion: "3.0.0", LatestVersion: "3.0.9", UpdateType: "patch"},
	}

	applied, err := updater.ApplyPatches(context.Background(), "/project", deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(applied) != 3 {
		t.Errorf("expected 3 applied, got %d", len(applied))
	}
	if len(runner.calls) != 3 {
		t.Errorf("expected 3 runner calls, got %d", len(runner.calls))
	}
}

func TestApplyPatches_MixedTypes(t *testing.T) {
	runner := &callTrackingRunner{}
	updater := NewUpdater(runner)

	deps := []dependency.Dependency{
		{Name: "vendor/patch-pkg", CurrentVersion: "1.0.0", LatestVersion: "1.0.1", UpdateType: "patch"},
		{Name: "vendor/minor-pkg", CurrentVersion: "1.0.0", LatestVersion: "1.1.0", UpdateType: "minor"},
		{Name: "vendor/major-pkg", CurrentVersion: "1.0.0", LatestVersion: "2.0.0", UpdateType: "major"},
	}

	applied, err := updater.ApplyPatches(context.Background(), "/project", deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(applied) != 1 {
		t.Errorf("expected 1 applied, got %d", len(applied))
	}
	if applied[0].Name != "vendor/patch-pkg" {
		t.Errorf("expected vendor/patch-pkg, got %s", applied[0].Name)
	}
	if len(runner.calls) != 1 {
		t.Errorf("expected 1 runner call, got %d", len(runner.calls))
	}
}

func TestApplyPatches_PartialFailure(t *testing.T) {
	runner := &callTrackingRunner{
		errs: map[int]error{
			1: errors.New("update failed"),
		},
	}
	updater := NewUpdater(runner)

	deps := []dependency.Dependency{
		{Name: "vendor/a", CurrentVersion: "1.0.0", LatestVersion: "1.0.1", UpdateType: "patch"},
		{Name: "vendor/b", CurrentVersion: "2.0.0", LatestVersion: "2.0.1", UpdateType: "patch"},
		{Name: "vendor/c", CurrentVersion: "3.0.0", LatestVersion: "3.0.1", UpdateType: "patch"},
	}

	applied, err := updater.ApplyPatches(context.Background(), "/project", deps)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if len(applied) != 2 {
		t.Errorf("expected 2 applied (succeeded ones), got %d", len(applied))
	}
}

func TestApplyPatches_AllFail(t *testing.T) {
	runner := &callTrackingRunner{
		errs: map[int]error{
			0: errors.New("fail 0"),
			1: errors.New("fail 1"),
		},
	}
	updater := NewUpdater(runner)

	deps := []dependency.Dependency{
		{Name: "vendor/a", CurrentVersion: "1.0.0", LatestVersion: "1.0.1", UpdateType: "patch"},
		{Name: "vendor/b", CurrentVersion: "2.0.0", LatestVersion: "2.0.1", UpdateType: "patch"},
	}

	applied, err := updater.ApplyPatches(context.Background(), "/project", deps)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if len(applied) != 0 {
		t.Errorf("expected 0 applied, got %d", len(applied))
	}
}

func TestApplyPatches_EmptyLatestVersion(t *testing.T) {
	runner := &callTrackingRunner{}
	updater := NewUpdater(runner)

	deps := []dependency.Dependency{
		{Name: "vendor/foo", CurrentVersion: "1.0.0", LatestVersion: "", UpdateType: "patch"},
	}

	applied, err := updater.ApplyPatches(context.Background(), "/project", deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if applied != nil {
		t.Errorf("expected nil applied, got %v", applied)
	}
	if len(runner.calls) != 0 {
		t.Errorf("expected no runner calls, got %d", len(runner.calls))
	}
}
