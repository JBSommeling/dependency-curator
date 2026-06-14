package gomod

import (
	"os"
	"path/filepath"
	"testing"
)

func TestProvider_Discover(t *testing.T) {
	dir := t.TempDir()
	goMod := `module example.com/myproject

go 1.24

require (
	github.com/stretchr/testify v1.9.0
	golang.org/x/text v0.14.0
	github.com/some/indirect v1.0.0 // indirect
)

require github.com/single/dep v2.3.0
`
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte(goMod), 0644)

	p := NewProvider()
	deps, err := p.Discover(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(deps) != 3 {
		t.Fatalf("expected 3 deps, got %d: %+v", len(deps), deps)
	}

	if deps[0].Name != "github.com/stretchr/testify" {
		t.Errorf("deps[0].Name = %q, want github.com/stretchr/testify", deps[0].Name)
	}
	if deps[0].CurrentVersion != "1.9.0" {
		t.Errorf("deps[0].CurrentVersion = %q, want 1.9.0", deps[0].CurrentVersion)
	}

	// Indirect should be skipped
	for _, d := range deps {
		if d.Name == "github.com/some/indirect" {
			t.Error("indirect dependency should be skipped")
		}
	}

	// Single-line require
	found := false
	for _, d := range deps {
		if d.Name == "github.com/single/dep" {
			found = true
			if d.CurrentVersion != "2.3.0" {
				t.Errorf("single/dep version = %q, want 2.3.0", d.CurrentVersion)
			}
		}
	}
	if !found {
		t.Error("single-line require dep not found")
	}
}

func TestProvider_Discover_NoDeps(t *testing.T) {
	dir := t.TempDir()
	goMod := `module example.com/myproject

go 1.24
`
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte(goMod), 0644)

	p := NewProvider()
	deps, err := p.Discover(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(deps) != 0 {
		t.Errorf("expected 0 deps, got %d", len(deps))
	}
}

func TestProvider_Discover_NoFile(t *testing.T) {
	dir := t.TempDir()
	p := NewProvider()
	_, err := p.Discover(dir)
	if err == nil {
		t.Error("expected error when go.mod doesn't exist")
	}
}

func TestProvider_Name(t *testing.T) {
	p := NewProvider()
	if p.Name() != "gomod" {
		t.Errorf("Name() = %q, want gomod", p.Name())
	}
}
