package updater

import (
	"context"
	"errors"
	"testing"

	"github.com/JBSommeling/dependency-curator/internal/dependency"
)

type mockRunner struct {
	calls []runCall
	errs  map[int]error // index -> error
}

type runCall struct {
	dir  string
	name string
	args []string
}

func (m *mockRunner) Run(ctx context.Context, dir string, name string, args ...string) ([]byte, error) {
	idx := len(m.calls)
	m.calls = append(m.calls, runCall{dir: dir, name: name, args: args})
	if err, ok := m.errs[idx]; ok {
		return nil, err
	}
	return nil, nil
}

func (m *mockRunner) RunAllowExit1(ctx context.Context, dir string, name string, args ...string) ([]byte, error) {
	return m.Run(ctx, dir, name, args...)
}

func TestApplyPatches_NoPatches(t *testing.T) {
	runner := &mockRunner{}
	u := New(runner)

	deps := []dependency.Dependency{
		{Name: "lodash", CurrentVersion: "1.0.0", LatestVersion: "2.0.0", UpdateType: "major"},
		{Name: "express", CurrentVersion: "4.0.0", LatestVersion: "4.1.0", UpdateType: "minor"},
	}

	applied, err := u.ApplyPatches(context.Background(), "/project", deps)

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if applied != nil {
		t.Errorf("expected nil applied, got %v", applied)
	}
	if len(runner.calls) != 0 {
		t.Errorf("expected no runner calls, got %d", len(runner.calls))
	}
}

func TestApplyPatches_SinglePatch(t *testing.T) {
	runner := &mockRunner{}
	u := New(runner)

	deps := []dependency.Dependency{
		{Name: "lodash", CurrentVersion: "1.0.0", LatestVersion: "1.0.1", UpdateType: "patch"},
	}

	applied, err := u.ApplyPatches(context.Background(), "/project", deps)

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if len(applied) != 1 {
		t.Fatalf("expected 1 applied dep, got %d", len(applied))
	}
	if len(runner.calls) != 1 {
		t.Fatalf("expected 1 runner call, got %d", len(runner.calls))
	}
	call := runner.calls[0]
	if call.dir != "/project" {
		t.Errorf("expected dir /project, got %s", call.dir)
	}
	if call.name != "npm" {
		t.Errorf("expected command npm, got %s", call.name)
	}
	if len(call.args) != 3 || call.args[0] != "install" || call.args[1] != "--" || call.args[2] != "lodash@1.0.1" {
		t.Errorf("expected args [install -- lodash@1.0.1], got %v", call.args)
	}
}

func TestApplyPatches_MultiplePatches(t *testing.T) {
	runner := &mockRunner{}
	u := New(runner)

	deps := []dependency.Dependency{
		{Name: "a", CurrentVersion: "1.0.0", LatestVersion: "1.0.1", UpdateType: "patch"},
		{Name: "b", CurrentVersion: "2.0.0", LatestVersion: "2.0.3", UpdateType: "patch"},
		{Name: "c", CurrentVersion: "3.0.0", LatestVersion: "3.0.2", UpdateType: "patch"},
	}

	applied, err := u.ApplyPatches(context.Background(), "/project", deps)

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if len(applied) != 3 {
		t.Errorf("expected 3 applied deps, got %d", len(applied))
	}
	if len(runner.calls) != 3 {
		t.Errorf("expected 3 runner calls, got %d", len(runner.calls))
	}
	expectedPkgs := []string{"a@1.0.1", "b@2.0.3", "c@3.0.2"}
	for i, call := range runner.calls {
		if len(call.args) < 3 || call.args[2] != expectedPkgs[i] {
			t.Errorf("call %d: expected pkg %s, got %v", i, expectedPkgs[i], call.args)
		}
	}
}

func TestApplyPatches_MixedTypes(t *testing.T) {
	runner := &mockRunner{}
	u := New(runner)

	deps := []dependency.Dependency{
		{Name: "patch-pkg", CurrentVersion: "1.0.0", LatestVersion: "1.0.1", UpdateType: "patch"},
		{Name: "minor-pkg", CurrentVersion: "1.0.0", LatestVersion: "1.1.0", UpdateType: "minor"},
		{Name: "major-pkg", CurrentVersion: "1.0.0", LatestVersion: "2.0.0", UpdateType: "major"},
	}

	applied, err := u.ApplyPatches(context.Background(), "/project", deps)

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if len(applied) != 1 {
		t.Errorf("expected 1 applied dep, got %d", len(applied))
	}
	if applied[0].Name != "patch-pkg" {
		t.Errorf("expected patch-pkg to be applied, got %s", applied[0].Name)
	}
	if len(runner.calls) != 1 {
		t.Errorf("expected 1 runner call, got %d", len(runner.calls))
	}
}

func TestApplyPatches_PartialFailure(t *testing.T) {
	installErr := errors.New("npm install failed")
	runner := &mockRunner{
		errs: map[int]error{
			1: installErr, // second call fails
		},
	}
	u := New(runner)

	deps := []dependency.Dependency{
		{Name: "a", CurrentVersion: "1.0.0", LatestVersion: "1.0.1", UpdateType: "patch"},
		{Name: "b", CurrentVersion: "2.0.0", LatestVersion: "2.0.1", UpdateType: "patch"},
		{Name: "c", CurrentVersion: "3.0.0", LatestVersion: "3.0.1", UpdateType: "patch"},
	}

	applied, err := u.ApplyPatches(context.Background(), "/project", deps)

	if err == nil {
		t.Error("expected an error, got nil")
	}
	if len(applied) != 2 {
		t.Errorf("expected 2 applied deps (a and c), got %d", len(applied))
	}
	if len(runner.calls) != 3 {
		t.Errorf("expected 3 runner calls, got %d", len(runner.calls))
	}
}

func TestApplyPatches_AllFail(t *testing.T) {
	installErr := errors.New("npm install failed")
	runner := &mockRunner{
		errs: map[int]error{
			0: installErr,
			1: installErr,
		},
	}
	u := New(runner)

	deps := []dependency.Dependency{
		{Name: "a", CurrentVersion: "1.0.0", LatestVersion: "1.0.1", UpdateType: "patch"},
		{Name: "b", CurrentVersion: "2.0.0", LatestVersion: "2.0.1", UpdateType: "patch"},
	}

	applied, err := u.ApplyPatches(context.Background(), "/project", deps)

	if err == nil {
		t.Error("expected an error, got nil")
	}
	if len(applied) != 0 {
		t.Errorf("expected 0 applied deps, got %d", len(applied))
	}
}

func TestApplyPatches_EmptyLatestVersion(t *testing.T) {
	runner := &mockRunner{}
	u := New(runner)

	deps := []dependency.Dependency{
		{Name: "a", CurrentVersion: "1.0.0", LatestVersion: "", UpdateType: "patch"},
	}

	applied, err := u.ApplyPatches(context.Background(), "/project", deps)

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if applied != nil {
		t.Errorf("expected nil applied, got %v", applied)
	}
	if len(runner.calls) != 0 {
		t.Errorf("expected no runner calls, got %d", len(runner.calls))
	}
}

func TestApplyUpdates_MinorUpdate(t *testing.T) {
	runner := &mockRunner{}
	u := New(runner)

	deps := []dependency.Dependency{
		{Name: "express", CurrentVersion: "4.0.0", LatestVersion: "4.1.0", UpdateType: "minor"},
	}

	applied, err := u.ApplyUpdates(context.Background(), "/project", deps)

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if len(applied) != 1 {
		t.Fatalf("expected 1 applied dep, got %d", len(applied))
	}
	if len(runner.calls) != 1 {
		t.Fatalf("expected 1 runner call, got %d", len(runner.calls))
	}
	call := runner.calls[0]
	if call.name != "npm" {
		t.Errorf("expected command npm, got %s", call.name)
	}
	if len(call.args) != 3 || call.args[0] != "install" || call.args[1] != "--" || call.args[2] != "express@4.1.0" {
		t.Errorf("expected args [install -- express@4.1.0], got %v", call.args)
	}
}

func TestApplyUpdates_MajorUpdate(t *testing.T) {
	runner := &mockRunner{}
	u := New(runner)

	deps := []dependency.Dependency{
		{Name: "lodash", CurrentVersion: "3.0.0", LatestVersion: "4.0.0", UpdateType: "major"},
	}

	applied, err := u.ApplyUpdates(context.Background(), "/project", deps)

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if len(applied) != 1 {
		t.Fatalf("expected 1 applied dep, got %d", len(applied))
	}
	if applied[0].Name != "lodash" {
		t.Errorf("expected lodash to be applied, got %s", applied[0].Name)
	}
}

func TestApplyUpdates_MixedTypes(t *testing.T) {
	runner := &mockRunner{}
	u := New(runner)

	deps := []dependency.Dependency{
		{Name: "patch-pkg", CurrentVersion: "1.0.0", LatestVersion: "1.0.1", UpdateType: "patch"},
		{Name: "minor-pkg", CurrentVersion: "1.0.0", LatestVersion: "1.1.0", UpdateType: "minor"},
		{Name: "major-pkg", CurrentVersion: "1.0.0", LatestVersion: "2.0.0", UpdateType: "major"},
	}

	applied, err := u.ApplyUpdates(context.Background(), "/project", deps)

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if len(applied) != 3 {
		t.Errorf("expected 3 applied deps (all types), got %d", len(applied))
	}
	if len(runner.calls) != 3 {
		t.Errorf("expected 3 runner calls, got %d", len(runner.calls))
	}
}

func TestApplyUpdates_EmptyLatestVersion(t *testing.T) {
	runner := &mockRunner{}
	u := New(runner)

	deps := []dependency.Dependency{
		{Name: "a", CurrentVersion: "1.0.0", LatestVersion: "", UpdateType: "minor"},
	}

	applied, err := u.ApplyUpdates(context.Background(), "/project", deps)

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if applied != nil {
		t.Errorf("expected nil applied, got %v", applied)
	}
	if len(runner.calls) != 0 {
		t.Errorf("expected no runner calls, got %d", len(runner.calls))
	}
}
