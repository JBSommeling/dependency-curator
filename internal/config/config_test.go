package config

import (
	"testing"
)

// minimalEnv sets the minimum required environment variables for Load() to succeed.
func minimalEnv(t *testing.T) {
	t.Helper()
	t.Setenv("INPUT_GITHUB_TOKEN", "test-token")
	t.Setenv("GITHUB_REPOSITORY", "owner/repo")
}

func TestLoad_DefaultsOnly(t *testing.T) {
	minimalEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if cfg.Token != "test-token" {
		t.Errorf("Token: got %q, want %q", cfg.Token, "test-token")
	}
	if cfg.Repository != "owner/repo" {
		t.Errorf("Repository: got %q, want %q", cfg.Repository, "owner/repo")
	}
	if cfg.Owner != "owner" {
		t.Errorf("Owner: got %q, want %q", cfg.Owner, "owner")
	}
	if cfg.Repo != "repo" {
		t.Errorf("Repo: got %q, want %q", cfg.Repo, "repo")
	}
	if cfg.BaseBranch != "" {
		t.Errorf("BaseBranch: got %q, want %q", cfg.BaseBranch, "")
	}
	if !cfg.AutoPatch {
		t.Error("AutoPatch: got false, want true (default)")
	}
	if !cfg.CreatePR {
		t.Error("CreatePR: got false, want true (default)")
	}
	if !cfg.IncludeDev {
		t.Error("IncludeDev: got false, want true (default)")
	}
	if cfg.ScheduleLabel != "weekly" {
		t.Errorf("ScheduleLabel: got %q, want %q", cfg.ScheduleLabel, "weekly")
	}
	if cfg.RiskThreshold != "high" {
		t.Errorf("RiskThreshold: got %q, want %q", cfg.RiskThreshold, "high")
	}
	if cfg.ProjectDir != "." {
		t.Errorf("ProjectDir: got %q, want %q", cfg.ProjectDir, ".")
	}
	if len(cfg.Labels) != 0 {
		t.Errorf("Labels: got %v, want empty", cfg.Labels)
	}
}

func TestLoad_AllOverrides(t *testing.T) {
	t.Setenv("INPUT_GITHUB_TOKEN", "override-token")
	t.Setenv("GITHUB_REPOSITORY", "myorg/myrepo")
	t.Setenv("INPUT_BASE_BRANCH", "develop")
	t.Setenv("INPUT_AUTO_PATCH", "false")
	t.Setenv("INPUT_CREATE_PR", "false")
	t.Setenv("INPUT_INCLUDE_DEV_DEPENDENCIES", "false")
	t.Setenv("INPUT_SCHEDULE_LABEL", "monthly")
	t.Setenv("INPUT_RISK_THRESHOLD", "low")
	t.Setenv("INPUT_PROJECT_DIR", "/workspace/myproject")
	t.Setenv("INPUT_LABELS", "dependencies,automated")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if cfg.Token != "override-token" {
		t.Errorf("Token: got %q, want %q", cfg.Token, "override-token")
	}
	if cfg.Owner != "myorg" {
		t.Errorf("Owner: got %q, want %q", cfg.Owner, "myorg")
	}
	if cfg.Repo != "myrepo" {
		t.Errorf("Repo: got %q, want %q", cfg.Repo, "myrepo")
	}
	if cfg.BaseBranch != "develop" {
		t.Errorf("BaseBranch: got %q, want %q", cfg.BaseBranch, "develop")
	}
	if cfg.AutoPatch {
		t.Error("AutoPatch: got true, want false")
	}
	if cfg.CreatePR {
		t.Error("CreatePR: got true, want false")
	}
	if cfg.IncludeDev {
		t.Error("IncludeDev: got true, want false")
	}
	if cfg.ScheduleLabel != "monthly" {
		t.Errorf("ScheduleLabel: got %q, want %q", cfg.ScheduleLabel, "monthly")
	}
	if cfg.RiskThreshold != "low" {
		t.Errorf("RiskThreshold: got %q, want %q", cfg.RiskThreshold, "low")
	}
	if cfg.ProjectDir != "/workspace/myproject" {
		t.Errorf("ProjectDir: got %q, want %q", cfg.ProjectDir, "/workspace/myproject")
	}
	if len(cfg.Labels) != 2 || cfg.Labels[0] != "dependencies" || cfg.Labels[1] != "automated" {
		t.Errorf("Labels: got %v, want [dependencies automated]", cfg.Labels)
	}
}

func TestLoad_MissingToken(t *testing.T) {
	t.Setenv("GITHUB_REPOSITORY", "owner/repo")
	// Ensure neither token env var is set
	t.Setenv("INPUT_GITHUB_TOKEN", "")
	t.Setenv("GITHUB_TOKEN", "")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for missing token, got nil")
	}
}

func TestLoad_TokenFallbackToGITHUB_TOKEN(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "fallback-token")
	t.Setenv("GITHUB_REPOSITORY", "owner/repo")
	// Ensure INPUT_GITHUB_TOKEN is not set
	t.Setenv("INPUT_GITHUB_TOKEN", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if cfg.Token != "fallback-token" {
		t.Errorf("Token: got %q, want %q", cfg.Token, "fallback-token")
	}
}

func TestLoad_MissingRepository(t *testing.T) {
	t.Setenv("INPUT_GITHUB_TOKEN", "test-token")
	t.Setenv("GITHUB_REPOSITORY", "")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for missing repository, got nil")
	}
}

func TestLoad_InvalidRepositoryFormat(t *testing.T) {
	t.Setenv("INPUT_GITHUB_TOKEN", "test-token")
	t.Setenv("GITHUB_REPOSITORY", "nodslashhere")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid repository format, got nil")
	}
}

func TestParseBool(t *testing.T) {
	cases := []struct {
		input string
		want  bool
	}{
		{"true", true},
		{"TRUE", true},
		{"True", true},
		{"1", true},
		{"yes", true},
		{"YES", true},
		{"false", false},
		{"FALSE", false},
		{"False", false},
		{"0", false},
		{"no", false},
		{"NO", false},
		{"", false},
		{"random", false},
	}

	for _, tc := range cases {
		got := parseBool(tc.input)
		if got != tc.want {
			t.Errorf("parseBool(%q) = %v, want %v", tc.input, got, tc.want)
		}
	}
}

func TestLoad_LabelsCommaSeparated(t *testing.T) {
	minimalEnv(t)
	t.Setenv("INPUT_LABELS", "  alpha , beta,, gamma  ,delta")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	want := []string{"alpha", "beta", "gamma", "delta"}
	if len(cfg.Labels) != len(want) {
		t.Fatalf("Labels length: got %d, want %d — labels: %v", len(cfg.Labels), len(want), cfg.Labels)
	}
	for i, l := range want {
		if cfg.Labels[i] != l {
			t.Errorf("Labels[%d]: got %q, want %q", i, cfg.Labels[i], l)
		}
	}
}

func TestUpdateBranch(t *testing.T) {
	cases := []struct {
		scheduleLabel string
		want          string
	}{
		{"weekly", "dependency-curator/weekly-updates"},
		{"monthly", "dependency-curator/monthly-updates"},
		{"daily", "dependency-curator/daily-updates"},
	}

	for _, tc := range cases {
		cfg := &Config{ScheduleLabel: tc.scheduleLabel}
		got := cfg.UpdateBranch()
		if got != tc.want {
			t.Errorf("UpdateBranch() with ScheduleLabel=%q: got %q, want %q", tc.scheduleLabel, got, tc.want)
		}
	}
}

func TestLoad_ProjectDirFallbackChain(t *testing.T) {
	// Case 1: INPUT_PROJECT_DIR takes priority
	t.Run("INPUT_PROJECT_DIR set", func(t *testing.T) {
		minimalEnv(t)
		t.Setenv("INPUT_PROJECT_DIR", "/from-input")
		t.Setenv("GITHUB_WORKSPACE", "/from-workspace")

		cfg, err := Load()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.ProjectDir != "/from-input" {
			t.Errorf("ProjectDir: got %q, want %q", cfg.ProjectDir, "/from-input")
		}
	})

	// Case 2: GITHUB_WORKSPACE is used when INPUT_PROJECT_DIR is absent
	t.Run("GITHUB_WORKSPACE fallback", func(t *testing.T) {
		minimalEnv(t)
		t.Setenv("INPUT_PROJECT_DIR", "")
		t.Setenv("GITHUB_WORKSPACE", "/from-workspace")

		cfg, err := Load()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.ProjectDir != "/from-workspace" {
			t.Errorf("ProjectDir: got %q, want %q", cfg.ProjectDir, "/from-workspace")
		}
	})

	// Case 3: Falls back to "." when both are absent
	t.Run("dot fallback", func(t *testing.T) {
		minimalEnv(t)
		t.Setenv("INPUT_PROJECT_DIR", "")
		t.Setenv("GITHUB_WORKSPACE", "")

		cfg, err := Load()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.ProjectDir != "." {
			t.Errorf("ProjectDir: got %q, want %q", cfg.ProjectDir, ".")
		}
	})
}
