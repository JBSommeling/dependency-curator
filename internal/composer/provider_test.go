package composer

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/JBSommeling/dependency-curator/internal/dependency"
)

func sortDeps(deps []dependency.Dependency) {
	sort.Slice(deps, func(i, j int) bool {
		return deps[i].Name < deps[j].Name
	})
}

func assertDep(t *testing.T, d dependency.Dependency, name, version string, isDev bool) {
	t.Helper()
	if d.Name != name {
		t.Errorf("Name: got %q, want %q", d.Name, name)
	}
	if d.CurrentVersion != version {
		t.Errorf("%s CurrentVersion: got %q, want %q", name, d.CurrentVersion, version)
	}
	if d.IsDev != isDev {
		t.Errorf("%s IsDev: got %v, want %v", name, d.IsDev, isDev)
	}
}

// copyFixture copies the named fixture file into a temp dir as composer.json and returns the dir.
func copyFixture(t *testing.T, fixturePath string) string {
	t.Helper()
	data, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("reading fixture %s: %v", fixturePath, err)
	}
	dir := t.TempDir()
	dest := filepath.Join(dir, "composer.json")
	if err := os.WriteFile(dest, data, 0644); err != nil {
		t.Fatalf("writing composer.json: %v", err)
	}
	return dir
}

func TestDiscover_Basic(t *testing.T) {
	p := NewComposerProvider()
	dir := copyFixture(t, "testdata/basic.json")
	deps, err := p.Discover(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(deps) != 3 {
		t.Fatalf("expected 3 deps, got %d: %v", len(deps), deps)
	}
	sortDeps(deps)
	// Sorted: guzzlehttp/guzzle, laravel/framework, phpunit/phpunit
	assertDep(t, deps[0], "guzzlehttp/guzzle", "7.2", false)
	assertDep(t, deps[1], "laravel/framework", "10.0", false)
	assertDep(t, deps[2], "phpunit/phpunit", "10.0", true)
}

func TestDiscover_Empty(t *testing.T) {
	p := NewComposerProvider()
	dir := copyFixture(t, "testdata/empty.json")
	deps, err := p.Discover(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(deps) != 0 {
		t.Fatalf("expected 0 deps, got %d", len(deps))
	}
}

func TestDiscover_ProdOnly(t *testing.T) {
	p := NewComposerProvider()
	dir := copyFixture(t, "testdata/prod_only.json")
	deps, err := p.Discover(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(deps) != 2 {
		t.Fatalf("expected 2 deps, got %d: %v", len(deps), deps)
	}
	for _, d := range deps {
		if d.IsDev {
			t.Errorf("dep %s should not be IsDev", d.Name)
		}
	}
	sortDeps(deps)
	// monolog/monolog, symfony/console
	assertDep(t, deps[0], "monolog/monolog", "3.0", false)
	assertDep(t, deps[1], "symfony/console", "6.3.0", false)
}

func TestDiscover_DevOnly(t *testing.T) {
	p := NewComposerProvider()
	dir := copyFixture(t, "testdata/dev_only.json")
	deps, err := p.Discover(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(deps) != 2 {
		t.Fatalf("expected 2 deps, got %d: %v", len(deps), deps)
	}
	for _, d := range deps {
		if !d.IsDev {
			t.Errorf("dep %s should be IsDev", d.Name)
		}
	}
	sortDeps(deps)
	// friendsofphp/php-cs-fixer, phpstan/phpstan
	assertDep(t, deps[0], "friendsofphp/php-cs-fixer", "3.0", true)
	assertDep(t, deps[1], "phpstan/phpstan", "1.10", true)
}

func TestDiscover_Malformed(t *testing.T) {
	p := NewComposerProvider()
	dir := copyFixture(t, "testdata/malformed.json")
	_, err := p.Discover(dir)
	if err == nil {
		t.Fatal("expected error for malformed JSON, got nil")
	}
}

func TestDiscover_MissingFile(t *testing.T) {
	p := NewComposerProvider()
	_, err := p.Discover("/nonexistent/path/that/does/not/exist")
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

func TestDiscover_PlatformReqs(t *testing.T) {
	p := NewComposerProvider()
	dir := copyFixture(t, "testdata/platform_reqs.json")
	deps, err := p.Discover(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(deps) != 1 {
		t.Fatalf("expected 1 dep, got %d: %v", len(deps), deps)
	}
	assertDep(t, deps[0], "guzzlehttp/guzzle", "7.0", false)
}

func TestProviderName(t *testing.T) {
	p := NewComposerProvider()
	if got := p.Name(); got != "composer" {
		t.Errorf("Name() = %q, want %q", got, "composer")
	}
}
