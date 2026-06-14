package gomod

import (
	"context"
	"errors"
	"testing"

	"github.com/JBSommeling/dependency-curator/internal/dependency"
)

func TestUpdater_ApplyPatches(t *testing.T) {
	runner := &mockRunner{responses: map[string]mockResp{
		"go get github.com/pkg/errors@v0.9.2": {output: nil},
		"go mod tidy":                          {output: nil},
	}}

	u := NewUpdater(runner)
	deps := []dependency.Dependency{
		{Name: "github.com/pkg/errors", LatestVersion: "0.9.2", UpdateType: "patch"},
		{Name: "golang.org/x/text", LatestVersion: "0.21.0", UpdateType: "minor"},
	}

	applied, err := u.ApplyPatches(context.Background(), "/test", deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(applied) != 1 {
		t.Fatalf("expected 1 applied, got %d", len(applied))
	}
	if applied[0].Name != "github.com/pkg/errors" {
		t.Errorf("applied[0].Name = %q, want github.com/pkg/errors", applied[0].Name)
	}
}

func TestUpdater_ApplyPatches_NoPatchDeps(t *testing.T) {
	runner := &mockRunner{responses: map[string]mockResp{}}

	u := NewUpdater(runner)
	deps := []dependency.Dependency{
		{Name: "golang.org/x/text", LatestVersion: "0.21.0", UpdateType: "minor"},
	}

	applied, err := u.ApplyPatches(context.Background(), "/test", deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(applied) != 0 {
		t.Errorf("expected 0 applied, got %d", len(applied))
	}
}

func TestUpdater_ApplyPatches_FailOnUpdate(t *testing.T) {
	runner := &mockRunner{responses: map[string]mockResp{
		"go get github.com/pkg/errors@v0.9.2": {err: errors.New("network error")},
		"go get github.com/other/pkg@v1.0.1":  {output: nil},
		"go mod tidy":                          {output: nil},
	}}

	u := NewUpdater(runner)
	deps := []dependency.Dependency{
		{Name: "github.com/pkg/errors", LatestVersion: "0.9.2", UpdateType: "patch"},
		{Name: "github.com/other/pkg", LatestVersion: "1.0.1", UpdateType: "patch"},
	}

	applied, err := u.ApplyPatches(context.Background(), "/test", deps)
	if err == nil {
		t.Error("expected error for failed update")
	}
	if len(applied) != 1 {
		t.Errorf("expected 1 applied (partial success), got %d", len(applied))
	}
	if len(applied) > 0 && applied[0].Name != "github.com/other/pkg" {
		t.Errorf("applied[0].Name = %q, want github.com/other/pkg", applied[0].Name)
	}
}

func TestUpdater_ApplyPatches_FailOnTidy(t *testing.T) {
	runner := &mockRunner{responses: map[string]mockResp{
		"go get github.com/pkg/errors@v0.9.2": {output: nil},
		"go mod tidy":                          {err: errors.New("tidy failed")},
	}}

	u := NewUpdater(runner)
	deps := []dependency.Dependency{
		{Name: "github.com/pkg/errors", LatestVersion: "0.9.2", UpdateType: "patch"},
	}

	applied, err := u.ApplyPatches(context.Background(), "/test", deps)
	if err == nil {
		t.Error("expected error on tidy failure")
	}
	if len(applied) != 1 {
		t.Errorf("expected 1 applied (tidy can fail after successful updates), got %d", len(applied))
	}
	if len(applied) > 0 && applied[0].Name != "github.com/pkg/errors" {
		t.Errorf("applied[0].Name = %q, want github.com/pkg/errors", applied[0].Name)
	}
}
