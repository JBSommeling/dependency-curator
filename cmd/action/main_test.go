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
		"npm install --ignore-scripts --no-audit": {output: nil},
		"npm outdated --json":                     {output: []byte(npmOutdated)},
		"npm audit --json":                        {output: []byte(`{"vulnerabilities":{}}`)},
		"npm install axios@1.6.8":                 {output: nil},
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
		"npm install --ignore-scripts --no-audit": {output: nil},
		"npm outdated --json":                     {output: []byte(npmOutdated)},
		"npm audit --json":                        {output: []byte(`{"vulnerabilities":{}}`)},
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

// TestRunWithDeps_ComposerOnly verifies that when only composer.json is present
// the orchestrator detects the composer ecosystem and creates a branch and PR.
func TestRunWithDeps_ComposerOnly(t *testing.T) {
	dir := t.TempDir()
	composerJSON := `{
		"name": "test/project",
		"require": {
			"guzzlehttp/guzzle": "^7.5.0",
			"phpunit/phpunit": "^9.0.0"
		}
	}`
	os.WriteFile(filepath.Join(dir, "composer.json"), []byte(composerJSON), 0644)
	os.WriteFile(filepath.Join(dir, "composer.lock"), []byte(`{}`), 0644)

	composerOutdated := `{
		"installed": [
			{"name": "guzzlehttp/guzzle", "version": "7.5.0", "latest": "7.5.3", "latest-status": "semver-safe-update"},
			{"name": "phpunit/phpunit", "version": "9.6.0", "latest": "10.5.0", "latest-status": "update-possible"}
		]
	}`

	ghMock := &mockGHClient{
		defaultBranch: "main",
		baseSHA:       "abc123",
		createdPR:     1,
		createdPRURL:  "https://github.com/test/repo/pull/1",
		commitSHA:     "def456",
	}

	runner := &mockCmdRunner{responses: map[string]mockResponse{
		"composer install --no-scripts --no-interaction":  {output: nil},
		"composer outdated --format=json --direct":        {output: []byte(composerOutdated)},
		"composer audit --format=json":                    {output: []byte(`{"advisories":{}}`)},
		"composer update --with guzzlehttp/guzzle:7.5.3": {output: nil},
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
}

// TestRunWithDeps_MixedEcosystems verifies that when both package.json and
// composer.json are present both ecosystems are discovered and combined into one PR.
func TestRunWithDeps_MixedEcosystems(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{
		"name":"test","version":"1.0.0",
		"dependencies":{"axios":"^1.6.0"}
	}`), 0644)
	os.WriteFile(filepath.Join(dir, "package-lock.json"), []byte(`{"lockfileVersion":3}`), 0644)
	os.WriteFile(filepath.Join(dir, "composer.json"), []byte(`{
		"name":"test/project",
		"require":{"monolog/monolog":"^3.0.0"}
	}`), 0644)
	os.WriteFile(filepath.Join(dir, "composer.lock"), []byte(`{}`), 0644)

	ghMock := &mockGHClient{
		defaultBranch: "main",
		baseSHA:       "abc123",
		createdPR:     1,
		createdPRURL:  "https://github.com/test/repo/pull/1",
		commitSHA:     "def456",
	}

	runner := &mockCmdRunner{responses: map[string]mockResponse{
		"npm install --ignore-scripts --no-audit":         {output: nil},
		"npm outdated --json":                             {output: []byte(`{"axios":{"current":"1.6.0","wanted":"1.6.8","latest":"1.6.8"}}`)},
		"npm audit --json":                                {output: []byte(`{"vulnerabilities":{}}`)},
		"npm install -- axios@1.6.8":                     {output: nil},
		"composer install --no-scripts --no-interaction": {output: nil},
		"composer outdated --format=json --direct":       {output: []byte(`{"installed":[{"name":"monolog/monolog","version":"3.0.0","latest":"3.5.0","latest-status":"semver-safe-update"}]}`)},
		"composer audit --format=json":                   {output: []byte(`{"advisories":{}}`)},
	}}

	d := &deps{runner: runner, ghClient: ghMock, httpClient: &mockHTTPClient{}}

	if err := runWithDeps(baseCfg(dir), d); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ghMock.createPRCalled {
		t.Error("should create PR with combined updates")
	}
	if !strings.Contains(ghMock.lastPRRequest.Body, "Dependency Curator Report") {
		t.Error("PR body should contain report")
	}
}

// TestRunWithDeps_NoEcosystems verifies that when no manifest files are present
// the orchestrator exits cleanly without touching GitHub.
func TestRunWithDeps_NoEcosystems(t *testing.T) {
	dir := t.TempDir()

	ghMock := &mockGHClient{defaultBranch: "main"}
	runner := &mockCmdRunner{responses: map[string]mockResponse{}}
	d := &deps{runner: runner, ghClient: ghMock, httpClient: &mockHTTPClient{}}

	if err := runWithDeps(baseCfg(dir), d); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ghMock.createBranchCalled {
		t.Error("should not create branch when no ecosystems detected")
	}
}

// TestRunWithDeps_GoModOnly verifies that when only go.mod is present
// the orchestrator detects the gomod ecosystem and creates a PR.
func TestRunWithDeps_GoModOnly(t *testing.T) {
	dir := t.TempDir()
	goMod := `module example.com/myproject

go 1.24

require (
	github.com/pkg/errors v0.9.1
	golang.org/x/text v0.14.0
)
`
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte(goMod), 0644)
	os.WriteFile(filepath.Join(dir, "go.sum"), []byte(""), 0644)

	goListOutput := `{"Path":"example.com/myproject","Version":"","Main":true}
{"Path":"github.com/pkg/errors","Version":"v0.9.1","Update":{"Version":"v0.9.2"},"Indirect":false}
{"Path":"golang.org/x/text","Version":"v0.14.0","Update":{"Version":"v0.21.0"},"Indirect":false}
`

	ghMock := &mockGHClient{
		defaultBranch: "main",
		baseSHA:       "abc123",
		createdPR:     1,
		createdPRURL:  "https://github.com/test/repo/pull/1",
		commitSHA:     "def456",
	}

	runner := &mockCmdRunner{responses: map[string]mockResponse{
		"go mod download":                     {output: nil},
		"go list -m -u -json all":             {output: []byte(goListOutput)},
		"govulncheck -json ./...":             {output: []byte("")},
		"go get github.com/pkg/errors@v0.9.2": {output: nil},
		"go mod tidy":                         {output: nil},
	}}

	d := &deps{runner: runner, ghClient: ghMock, httpClient: &mockHTTPClient{}}

	if err := runWithDeps(baseCfg(dir), d); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ghMock.createBranchCalled {
		t.Error("should create branch for updates")
	}
	if !ghMock.commitFilesCalled {
		t.Error("should commit patch updates")
	}
	if !ghMock.createPRCalled {
		t.Error("should create PR for minor/major updates")
	}
}

// TestRunWithDeps_AllEcosystems verifies that npm, composer, and gomod
// can all be detected and combined into a single PR.
func TestRunWithDeps_AllEcosystems(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{
		"name":"test","version":"1.0.0",
		"dependencies":{"axios":"^1.6.0"}
	}`), 0644)
	os.WriteFile(filepath.Join(dir, "package-lock.json"), []byte(`{"lockfileVersion":3}`), 0644)
	os.WriteFile(filepath.Join(dir, "composer.json"), []byte(`{
		"name":"test/project",
		"require":{"monolog/monolog":"^3.0.0"}
	}`), 0644)
	os.WriteFile(filepath.Join(dir, "composer.lock"), []byte(`{}`), 0644)
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte(`module example.com/test

go 1.24

require github.com/pkg/errors v0.9.1
`), 0644)
	os.WriteFile(filepath.Join(dir, "go.sum"), []byte(""), 0644)

	ghMock := &mockGHClient{
		defaultBranch: "main",
		baseSHA:       "abc123",
		createdPR:     1,
		createdPRURL:  "https://github.com/test/repo/pull/1",
		commitSHA:     "def456",
	}

	runner := &mockCmdRunner{responses: map[string]mockResponse{
		"npm install --ignore-scripts --no-audit":         {output: nil},
		"npm outdated --json":                             {output: []byte(`{"axios":{"current":"1.6.0","wanted":"1.6.8","latest":"1.6.8"}}`)},
		"npm audit --json":                                {output: []byte(`{"vulnerabilities":{}}`)},
		"npm install -- axios@1.6.8":                     {output: nil},
		"composer install --no-scripts --no-interaction": {output: nil},
		"composer outdated --format=json --direct":       {output: []byte(`{"installed":[{"name":"monolog/monolog","version":"3.0.0","latest":"3.5.0","latest-status":"semver-safe-update"}]}`)},
		"composer audit --format=json":                   {output: []byte(`{"advisories":{}}`)},
		"go mod download":                                 {output: nil},
		"go list -m -u -json all": {output: []byte(`{"Path":"example.com/test","Version":"","Main":true}
{"Path":"github.com/pkg/errors","Version":"v0.9.1","Update":{"Version":"v0.9.2"},"Indirect":false}
`)},
		"govulncheck -json ./...":              {output: []byte("")},
		"go get github.com/pkg/errors@v0.9.2": {output: nil},
		"go mod tidy":                          {output: nil},
	}}

	d := &deps{runner: runner, ghClient: ghMock, httpClient: &mockHTTPClient{}}

	if err := runWithDeps(baseCfg(dir), d); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ghMock.createPRCalled {
		t.Error("should create PR with combined updates from all ecosystems")
	}
	if !strings.Contains(ghMock.lastPRRequest.Body, "Dependency Curator Report") {
		t.Error("PR body should contain report")
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
