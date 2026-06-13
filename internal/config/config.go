package config

import (
	"fmt"
	"os"
	"strings"
)

type Config struct {
	Token          string
	Repository     string   // owner/repo
	Owner          string
	Repo           string
	BaseBranch     string
	AutoPatch      bool
	CreatePR       bool
	IncludeDev     bool
	ScheduleLabel  string
	RiskThreshold  string
	ProjectDir     string
	Labels         []string
}

func Load() (*Config, error) {
	token := getEnv("INPUT_GITHUB_TOKEN", "")
	if token == "" {
		token = os.Getenv("GITHUB_TOKEN")
	}
	if token == "" {
		return nil, fmt.Errorf("github token is required: set INPUT_GITHUB_TOKEN or GITHUB_TOKEN")
	}

	repo := os.Getenv("GITHUB_REPOSITORY")
	if repo == "" {
		return nil, fmt.Errorf("GITHUB_REPOSITORY is required")
	}

	parts := strings.SplitN(repo, "/", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid GITHUB_REPOSITORY format: %s", repo)
	}

	projectDir := getEnv("INPUT_PROJECT_DIR", "")
	if projectDir == "" {
		projectDir = os.Getenv("GITHUB_WORKSPACE")
	}
	if projectDir == "" {
		projectDir = "."
	}

	labelsStr := getEnv("INPUT_LABELS", "")
	var labels []string
	if labelsStr != "" {
		for _, l := range strings.Split(labelsStr, ",") {
			l = strings.TrimSpace(l)
			if l != "" {
				labels = append(labels, l)
			}
		}
	}

	return &Config{
		Token:         token,
		Repository:    repo,
		Owner:         parts[0],
		Repo:          parts[1],
		BaseBranch:    getEnv("INPUT_BASE_BRANCH", ""),
		AutoPatch:     parseBool(getEnv("INPUT_AUTO_PATCH", "true")),
		CreatePR:      parseBool(getEnv("INPUT_CREATE_PR", "true")),
		IncludeDev:    parseBool(getEnv("INPUT_INCLUDE_DEV_DEPENDENCIES", "true")),
		ScheduleLabel: getEnv("INPUT_SCHEDULE_LABEL", "weekly"),
		RiskThreshold: getEnv("INPUT_RISK_THRESHOLD", "high"),
		ProjectDir:    projectDir,
		Labels:        labels,
	}, nil
}

func (c *Config) UpdateBranch() string {
	return "dependency-curator/" + c.ScheduleLabel + "-updates"
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func parseBool(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	return s == "true" || s == "1" || s == "yes"
}
