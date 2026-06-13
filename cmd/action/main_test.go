package main

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/JBSommeling/dependency-curator/internal/config"
	ghpkg "github.com/JBSommeling/dependency-curator/internal/github"
)

// mockGHClient satisfies ghClientInterface for tests.
type mockGHClient struct {
	defaultBranch string
	baseSHA       string
	branchExists  bool
	existingPR    int
	createdPR     int
	createdPRURL  string
	commitSHA     string

	createBranchCalled bool
	commitFilesCalled  bool
	createPRCalled     bool
	updatePRCalled     bool
	lastPRRequest      ghpkg.PRRequest
}

func (m *mockGHClient) GetDefaultBranch(_ context.Context, _, _ string) (string, error) {
	return m.defaultBranch, nil
}
func (m *mockGHClient) GetRef(_ context.Context, _, _, _ string) (string, error) {
	return m.baseSHA, nil
}
func (m *mockGHClient) BranchExists(_ context.Context, _, _, _ string) (bool, error) {
	return m.branchExists, nil
}
func (m *mockGHClient) CreateBranch(_ context.Context, _, _, _, _ string) error {
	m.createBranchCalled = true
	return nil
}
func (m *mockGHClient) UpdateRef(_ context.Context, _, _, _, _ string) error {
	return nil
}
func (m *mockGHClient) CommitFiles(_ context.Context, _, _, _, _ string, _ map[string][]byte) (string, error) {
	m.commitFilesCalled = true
	return m.commitSHA, nil
}
func (m *mockGHClient) FindOpenPR(_ context.Context, _, _, _, _ string) (int, bool, error) {
	if m.existingPR > 0 {
		return m.existingPR, true, nil
	}
	return 0, false, nil
}
func (m *mockGHClient) CreatePR(_ context.Context, _, _ string, pr ghpkg.PRRequest) (string, int, error) {
	m.createPRCalled = true
	m.lastPRRequest = pr
	return m.createdPRURL, m.createdPR, nil
}
func (m *mockGHClient) UpdatePR(_ context.Context, _, _ string, _ int, pr ghpkg.PRRequest) error {
	m.updatePRCalled = true
	m.lastPRRequest = pr
	return nil
}
func (m *mockGHClient) AddLabels(_ context.Context, _, _ string, _ int, _ []string) error {
	return nil
}

// mockCmdRunner satisfies exec.CommandRunner for tests.
type mockCmdRunner struct {
	responses map[string]mockResponse
}

type mockResponse struct {
	output []byte
	err    error
}

func (m *mockCmdRunner) Run(_ context.Context, _ string, name string, args ...string) ([]byte, error) {
	key := name + " " + strings.Join(args, " ")
	if resp, ok := m.responses[key]; ok {
		return resp.output, resp.err
	}
	return []byte("{}"), nil
}

func (m *mockCmdRunner) RunAllowExit1(ctx context.Context, dir string, name string, args ...string) ([]byte, error) {
	return m.Run(ctx, dir, name, args...)
}

// mockHTTPClient satisfies changelog.HTTPClient for tests; returns 404 so no
// changelog enrichment is attempted.
type mockHTTPClient struct{}

func (m *mockHTTPClient) Do(_ *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: http.StatusNotFound,
		Body:       io.NopCloser(strings.NewReader("{}")),
	}, nil
}

// baseCfg returns a minimal config pointing at the given project directory.
func baseCfg(dir string) *config.Config {
	return &config.Config{
		Token:         "test-token",
		Owner:         "test",
		Repo:          "repo",
		BaseBranch:    "main",
		AutoPatch:     true,
		CreatePR:      true,
		IncludeDev:    true,
		ScheduleLabel: "weekly",
		ProjectDir:    dir,
	}
}

// TestRunWithDeps_NoDeps verifies that when package.json has no dependencies
// the orchestrator exits cleanly without touching GitHub.
func TestRunWithDeps_NoDeps(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"name":"test","version":"1.0.0"}`), 0644); err != nil {
		t.Fatal(err)
	}

	ghMock := &mockGHClient{defaultBranch: "main"}
	runner := &mockCmdRunner{responses: map[string]mockResponse{
		"npm outdated --json": {output: []byte("{}")},
		"npm audit --json":    {output: []byte(`{"vulnerabilities":{}}`)},
	}}

	d := &deps{runner: runner, ghClient: ghMock, httpClient: &mockHTTPClient{}}

	if err := runWithDeps(baseCfg(dir), d); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ghMock.createBranchCalled {
		t.Error("should not create branch when no dependencies")
	}
	if ghMock.createPRCalled {
		t.Error("should not create PR when no dependencies")
	}
}

// TestRunWithDeps_WithUpdates verifies that when npm outdated reports both
// patch and major updates a branch is created and a PR is opened.
func TestRunWithDeps_WithUpdates(t *testing.T) {
	dir := t.TempDir()
	pkgJSON := `{
		"name":"test","version":"1.0.0",
		"dependencies":{"axios":"^1.6.0","webpack":"^4.0.0"}
	}`
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(pkgJSON), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "package-lock.json"), []byte(`{"lockfileVersion":3}`), 0644); err != nil {
		t.Fatal(err)
	}

	npmOutdated := `{
		"axios":   {"current":"1.6.0","wanted":"1.6.8","latest":"1.6.8"},
		"webpack": {"current":"4.0.0","wanted":"4.0.0","latest":"5.89.0"}
	}`

	ghMock := &mockGHClient{
		defaultBranch: "main",
		baseSHA:       "abc123",
		branchExists:  false,
		createdPR:     1,
		createdPRURL:  "https://github.com/test/repo/pull/1",
		commitSHA:     "def456",
	}

	runner := &mockCmdRunner{responses: map[string]mockResponse{
		"npm outdated --json":      {output: []byte(npmOutdated)},
		"npm audit --json":         {output: []byte(`{"vulnerabilities":{}}`)},
		"npm install axios@1.6.8":  {output: nil},
	}}

	d := &deps{runner: runner, ghClient: ghMock, httpClient: &mockHTTPClient{}}

	if err := runWithDeps(baseCfg(dir), d); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ghMock.createBranchCalled {
		t.Error("should create branch for updates")
	}
	if !ghMock.createPRCalled {
		t.Error("should create PR for major updates")
	}
	if !strings.Contains(ghMock.lastPRRequest.Body, "Dependency Curator Report") {
		t.Errorf("PR body should contain report header, got:\n%s", ghMock.lastPRRequest.Body)
	}
}

// TestRunWithDeps_ExistingPR verifies that when a PR already exists the
// orchestrator updates it instead of creating a new one.
func TestRunWithDeps_ExistingPR(t *testing.T) {
	dir := t.TempDir()
	pkgJSON := `{
		"name":"test","version":"1.0.0",
		"dependencies":{"express":"^4.17.0"}
	}`
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(pkgJSON), 0644); err != nil {
		t.Fatal(err)
	}

	npmOutdated := `{"express":{"current":"4.17.0","wanted":"4.17.1","latest":"4.18.2"}}`

	ghMock := &mockGHClient{
		defaultBranch: "main",
		baseSHA:       "abc123",
		branchExists:  true,
		existingPR:    42,
	}

	runner := &mockCmdRunner{responses: map[string]mockResponse{
		"npm outdated --json": {output: []byte(npmOutdated)},
		"npm audit --json":    {output: []byte(`{"vulnerabilities":{}}`)},
	}}

	d := &deps{runner: runner, ghClient: ghMock, httpClient: &mockHTTPClient{}}

	if err := runWithDeps(baseCfg(dir), d); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ghMock.createPRCalled {
		t.Error("should not create new PR when one already exists")
	}
	if !ghMock.updatePRCalled {
		t.Error("should update the existing PR")
	}
}

// TestSetOutput verifies that setOutput writes the key and value to the file
// named by GITHUB_OUTPUT.
func TestSetOutput(t *testing.T) {
	f := filepath.Join(t.TempDir(), "github_output")
	if err := os.WriteFile(f, nil, 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GITHUB_OUTPUT", f)

	setOutput("test_key", "test_value")

	data, err := os.ReadFile(f)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !strings.Contains(content, "test_key") {
		t.Errorf("output file should contain key; got:\n%s", content)
	}
	if !strings.Contains(content, "test_value") {
		t.Errorf("output file should contain value; got:\n%s", content)
	}
}
