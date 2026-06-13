package changelog

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

type NpmRegistryProvider struct {
	client      HTTPClient
	registryURL string
}

func NewNpmRegistryProvider(client HTTPClient) *NpmRegistryProvider {
	return &NpmRegistryProvider{
		client:      client,
		registryURL: "https://registry.npmjs.org",
	}
}

type npmPackageInfo struct {
	Repository struct {
		Type string `json:"type"`
		URL  string `json:"url"`
	} `json:"repository"`
}

func (p *NpmRegistryProvider) FetchChangelog(pkg string, fromVer, toVer string) (*ChangelogInfo, error) {
	info := &ChangelogInfo{
		PackageName: pkg,
		FromVersion: fromVer,
		ToVersion:   toVer,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	url := fmt.Sprintf("%s/%s", p.registryURL, pkg)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return info, fmt.Errorf("creating request: %w", err)
	}
	// Only fetch metadata, not all versions
	req.Header.Set("Accept", "application/vnd.npm.install-v1+json")

	resp, err := p.client.Do(req)
	if err != nil {
		return info, fmt.Errorf("fetching package info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return info, nil // Not available, but not an error
	}

	var pkgInfo npmPackageInfo
	if err := json.NewDecoder(resp.Body).Decode(&pkgInfo); err != nil {
		return info, nil // Can't parse, treat as unavailable
	}

	repoURL := pkgInfo.Repository.URL
	if repoURL == "" {
		return info, nil
	}

	ghOwnerRepo := extractGitHubRepo(repoURL)
	if ghOwnerRepo == "" {
		return info, nil
	}

	info.Available = true
	info.ReleaseNotesURL = fmt.Sprintf("https://github.com/%s/releases/tag/v%s", ghOwnerRepo, toVer)
	info.ChangelogURL = fmt.Sprintf("https://github.com/%s/blob/main/CHANGELOG.md", ghOwnerRepo)

	return info, nil
}

func extractGitHubRepo(repoURL string) string {
	// Handle various formats:
	// git+https://github.com/owner/repo.git
	// https://github.com/owner/repo.git
	// git://github.com/owner/repo.git
	// git@github.com:owner/repo.git
	// github:owner/repo

	repoURL = strings.TrimPrefix(repoURL, "git+")
	repoURL = strings.TrimPrefix(repoURL, "git://")
	repoURL = strings.TrimPrefix(repoURL, "https://")
	repoURL = strings.TrimPrefix(repoURL, "http://")
	repoURL = strings.TrimSuffix(repoURL, ".git")

	// Handle git@github.com:owner/repo
	if strings.HasPrefix(repoURL, "git@github.com:") {
		repoURL = strings.TrimPrefix(repoURL, "git@github.com:")
		parts := strings.SplitN(repoURL, "/", 2)
		if len(parts) == 2 {
			return parts[0] + "/" + parts[1]
		}
		return ""
	}

	// Handle github:owner/repo
	if strings.HasPrefix(repoURL, "github:") {
		return strings.TrimPrefix(repoURL, "github:")
	}

	// Handle github.com/owner/repo
	if strings.HasPrefix(repoURL, "github.com/") {
		path := strings.TrimPrefix(repoURL, "github.com/")
		parts := strings.SplitN(path, "/", 3)
		if len(parts) >= 2 {
			return parts[0] + "/" + parts[1]
		}
	}

	return ""
}
