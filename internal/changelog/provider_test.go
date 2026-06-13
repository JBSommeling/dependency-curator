package changelog

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func makeProvider(t *testing.T, handler http.HandlerFunc) (*NpmRegistryProvider, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	p := NewNpmRegistryProvider(srv.Client())
	p.registryURL = srv.URL
	return p, srv
}

func TestFetchChangelog_GitHubHTTPS(t *testing.T) {
	p, srv := makeProvider(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"repository":{"type":"git","url":"https://github.com/owner/repo.git"}}`))
	})
	defer srv.Close()

	info, err := p.FetchChangelog(context.Background(), "mypackage", "1.0.0", "2.0.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !info.Available {
		t.Error("expected Available to be true")
	}
	if info.ReleaseNotesURL != "https://github.com/owner/repo/releases/tag/v2.0.0" {
		t.Errorf("unexpected ReleaseNotesURL: %s", info.ReleaseNotesURL)
	}
	if info.ChangelogURL != "https://github.com/owner/repo/blob/main/CHANGELOG.md" {
		t.Errorf("unexpected ChangelogURL: %s", info.ChangelogURL)
	}
}

func TestFetchChangelog_GitPlusHTTPS(t *testing.T) {
	p, srv := makeProvider(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"repository":{"type":"git","url":"git+https://github.com/owner/repo.git"}}`))
	})
	defer srv.Close()

	info, err := p.FetchChangelog(context.Background(), "mypackage", "1.0.0", "2.0.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !info.Available {
		t.Error("expected Available to be true")
	}
	if info.ReleaseNotesURL != "https://github.com/owner/repo/releases/tag/v2.0.0" {
		t.Errorf("unexpected ReleaseNotesURL: %s", info.ReleaseNotesURL)
	}
	if info.ChangelogURL != "https://github.com/owner/repo/blob/main/CHANGELOG.md" {
		t.Errorf("unexpected ChangelogURL: %s", info.ChangelogURL)
	}
}

func TestFetchChangelog_GitAtFormat(t *testing.T) {
	p, srv := makeProvider(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"repository":{"type":"git","url":"git@github.com:owner/repo.git"}}`))
	})
	defer srv.Close()

	info, err := p.FetchChangelog(context.Background(), "mypackage", "1.0.0", "2.0.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !info.Available {
		t.Error("expected Available to be true")
	}
	if info.ReleaseNotesURL != "https://github.com/owner/repo/releases/tag/v2.0.0" {
		t.Errorf("unexpected ReleaseNotesURL: %s", info.ReleaseNotesURL)
	}
	if info.ChangelogURL != "https://github.com/owner/repo/blob/main/CHANGELOG.md" {
		t.Errorf("unexpected ChangelogURL: %s", info.ChangelogURL)
	}
}

func TestFetchChangelog_GitHubShorthand(t *testing.T) {
	p, srv := makeProvider(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"repository":{"type":"git","url":"github:owner/repo"}}`))
	})
	defer srv.Close()

	info, err := p.FetchChangelog(context.Background(), "mypackage", "1.0.0", "2.0.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !info.Available {
		t.Error("expected Available to be true")
	}
	if info.ReleaseNotesURL != "https://github.com/owner/repo/releases/tag/v2.0.0" {
		t.Errorf("unexpected ReleaseNotesURL: %s", info.ReleaseNotesURL)
	}
	if info.ChangelogURL != "https://github.com/owner/repo/blob/main/CHANGELOG.md" {
		t.Errorf("unexpected ChangelogURL: %s", info.ChangelogURL)
	}
}

func TestFetchChangelog_NoRepository(t *testing.T) {
	p, srv := makeProvider(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"name":"mypackage"}`))
	})
	defer srv.Close()

	info, err := p.FetchChangelog(context.Background(), "mypackage", "1.0.0", "2.0.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.Available {
		t.Error("expected Available to be false when no repository field")
	}
}

func TestFetchChangelog_NonGitHubRepo(t *testing.T) {
	p, srv := makeProvider(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"repository":{"type":"git","url":"https://gitlab.com/owner/repo.git"}}`))
	})
	defer srv.Close()

	info, err := p.FetchChangelog(context.Background(), "mypackage", "1.0.0", "2.0.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.Available {
		t.Error("expected Available to be false for non-GitHub repo")
	}
}

func TestFetchChangelog_404(t *testing.T) {
	p, srv := makeProvider(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	defer srv.Close()

	info, err := p.FetchChangelog(context.Background(), "nonexistent-pkg", "1.0.0", "2.0.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.Available {
		t.Error("expected Available to be false on 404")
	}
}

func TestFetchChangelog_InvalidJSON(t *testing.T) {
	p, srv := makeProvider(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`not valid json{{{`))
	})
	defer srv.Close()

	info, err := p.FetchChangelog(context.Background(), "mypackage", "1.0.0", "2.0.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.Available {
		t.Error("expected Available to be false on invalid JSON")
	}
}

func TestFetchChangelog_NetworkError(t *testing.T) {
	p, srv := makeProvider(t, func(w http.ResponseWriter, r *http.Request) {
		// handler never used; server is closed before request
	})
	srv.Close() // Close immediately so the request fails

	_, err := p.FetchChangelog(context.Background(), "mypackage", "1.0.0", "2.0.0")
	if err == nil {
		t.Error("expected error on network failure, got nil")
	}
}
