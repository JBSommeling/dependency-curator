package dependency

import (
	"os"
	"sort"
	"testing"
)

func sortDeps(deps []Dependency) {
	sort.Slice(deps, func(i, j int) bool {
		return deps[i].Name < deps[j].Name
	})
}

func findDep(deps []Dependency, name string) (Dependency, bool) {
	for _, d := range deps {
		if d.Name == name {
			return d, true
		}
	}
	return Dependency{}, false
}

// stageFixture copies a testdata fixture into a temp dir as package.json,
// returning the temp dir path.
func stageFixture(t *testing.T, fixture string) string {
	t.Helper()
	data, err := os.ReadFile("testdata/" + fixture)
	if err != nil {
		t.Fatalf("reading fixture %s: %v", fixture, err)
	}
	dir := t.TempDir()
	if err := os.WriteFile(dir+"/package.json", data, 0644); err != nil {
		t.Fatalf("writing package.json: %v", err)
	}
	return dir
}

func TestDiscover_Basic(t *testing.T) {
	p := NewPackageJSONProvider()
	dir := stageFixture(t, "basic.json")

	deps, err := p.Discover(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(deps) != 4 {
		t.Fatalf("expected 4 deps, got %d", len(deps))
	}

	checks := map[string]struct {
		version string
		isDev   bool
	}{
		"axios":   {"1.6.0", false},
		"eslint":  {"8.50.0", true},
		"express": {"4.18.2", false},
		"jest":    {"29.0.0", true},
	}

	for name, want := range checks {
		d, ok := findDep(deps, name)
		if !ok {
			t.Errorf("dep %q not found", name)
			continue
		}
		if d.CurrentVersion != want.version {
			t.Errorf("%s: version = %q, want %q", name, d.CurrentVersion, want.version)
		}
		if d.IsDev != want.isDev {
			t.Errorf("%s: IsDev = %v, want %v", name, d.IsDev, want.isDev)
		}
	}
}

func TestDiscover_Empty(t *testing.T) {
	p := NewPackageJSONProvider()
	dir := stageFixture(t, "empty.json")

	deps, err := p.Discover(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(deps) != 0 {
		t.Errorf("expected 0 deps, got %d", len(deps))
	}
}

func TestDiscover_ProdOnly(t *testing.T) {
	p := NewPackageJSONProvider()
	dir := stageFixture(t, "prod_only.json")

	deps, err := p.Discover(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(deps) != 2 {
		t.Fatalf("expected 2 deps, got %d", len(deps))
	}
	for _, d := range deps {
		if d.IsDev {
			t.Errorf("dep %q should not be dev (IsDev=true)", d.Name)
		}
	}
}

func TestDiscover_DevOnly(t *testing.T) {
	p := NewPackageJSONProvider()
	dir := stageFixture(t, "dev_only.json")

	deps, err := p.Discover(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(deps) != 2 {
		t.Fatalf("expected 2 deps, got %d", len(deps))
	}
	for _, d := range deps {
		if !d.IsDev {
			t.Errorf("dep %q should be dev (IsDev=false)", d.Name)
		}
	}
}

func TestDiscover_Malformed(t *testing.T) {
	p := NewPackageJSONProvider()
	dir := stageFixture(t, "malformed.json")

	_, err := p.Discover(dir)
	if err == nil {
		t.Fatal("expected error for malformed JSON, got nil")
	}
}

func TestDiscover_MissingFile(t *testing.T) {
	p := NewPackageJSONProvider()

	_, err := p.Discover("/nonexistent/path/that/does/not/exist")
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

func TestProviderName(t *testing.T) {
	p := NewPackageJSONProvider()
	if got := p.Name(); got != "npm" {
		t.Errorf("Name() = %q, want %q", got, "npm")
	}
}

// Ensure sortDeps is exercised (used as utility, tested implicitly via Basic test).
func TestSortDeps(t *testing.T) {
	deps := []Dependency{
		{Name: "zebra"},
		{Name: "apple"},
		{Name: "mango"},
	}
	sortDeps(deps)
	expected := []string{"apple", "mango", "zebra"}
	for i, name := range expected {
		if deps[i].Name != name {
			t.Errorf("sorted[%d] = %q, want %q", i, deps[i].Name, name)
		}
	}
}
