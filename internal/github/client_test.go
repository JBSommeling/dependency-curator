package github

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

const testToken = "test-token"

func newTestClient(t *testing.T, handler http.Handler) (*Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	client := NewClientWithBaseURL(srv.Client(), testToken, srv.URL)
	return client, srv
}

func assertAuthHeader(t *testing.T, r *http.Request) {
	t.Helper()
	got := r.Header.Get("Authorization")
	want := "Bearer " + testToken
	if got != want {
		t.Errorf("Authorization header = %q, want %q", got, want)
	}
}

func TestGetDefaultBranch(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		if r.URL.Path != "/repos/owner/repo" {
			t.Errorf("path = %s, want /repos/owner/repo", r.URL.Path)
		}
		assertAuthHeader(t, r)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"default_branch": "main"})
	})

	client, _ := newTestClient(t, handler)
	branch, err := client.GetDefaultBranch(context.Background(), "owner", "repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if branch != "main" {
		t.Errorf("branch = %q, want %q", branch, "main")
	}
}

func TestGetRef(t *testing.T) {
	const wantSHA = "abc123def456"
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		if r.URL.Path != "/repos/owner/repo/git/ref/heads/main" {
			t.Errorf("path = %s, want /repos/owner/repo/git/ref/heads/main", r.URL.Path)
		}
		assertAuthHeader(t, r)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"object": map[string]string{"sha": wantSHA},
		})
	})

	client, _ := newTestClient(t, handler)
	sha, err := client.GetRef(context.Background(), "owner", "repo", "heads/main")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sha != wantSHA {
		t.Errorf("sha = %q, want %q", sha, wantSHA)
	}
}

func TestBranchExists_True(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertAuthHeader(t, r)
		w.WriteHeader(http.StatusOK)
	})

	client, _ := newTestClient(t, handler)
	exists, err := client.BranchExists(context.Background(), "owner", "repo", "feature-branch")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !exists {
		t.Error("expected branch to exist, got false")
	}
}

func TestBranchExists_False(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertAuthHeader(t, r)
		w.WriteHeader(http.StatusNotFound)
	})

	client, _ := newTestClient(t, handler)
	exists, err := client.BranchExists(context.Background(), "owner", "repo", "nonexistent-branch")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exists {
		t.Error("expected branch to not exist, got true")
	}
}

func TestCreateBranch(t *testing.T) {
	const wantRef = "refs/heads/new-branch"
	const wantSHA = "deadbeef"

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/repos/owner/repo/git/refs" {
			t.Errorf("path = %s, want /repos/owner/repo/git/refs", r.URL.Path)
		}
		assertAuthHeader(t, r)

		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decoding body: %v", err)
		}
		if body["ref"] != wantRef {
			t.Errorf("body ref = %q, want %q", body["ref"], wantRef)
		}
		if body["sha"] != wantSHA {
			t.Errorf("body sha = %q, want %q", body["sha"], wantSHA)
		}

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{"ref": wantRef})
	})

	client, _ := newTestClient(t, handler)
	err := client.CreateBranch(context.Background(), "owner", "repo", "new-branch", wantSHA)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFindOpenPR_Found(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		assertAuthHeader(t, r)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]map[string]interface{}{
			{"Number": 42, "HTMLURL": "https://github.com/owner/repo/pull/42", "State": "open"},
		})
	})

	client, _ := newTestClient(t, handler)
	number, found, err := client.FindOpenPR(context.Background(), "owner", "repo", "feature", "main")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !found {
		t.Error("expected PR to be found")
	}
	if number != 42 {
		t.Errorf("PR number = %d, want 42", number)
	}
}

func TestFindOpenPR_NotFound(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertAuthHeader(t, r)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]map[string]interface{}{})
	})

	client, _ := newTestClient(t, handler)
	_, found, err := client.FindOpenPR(context.Background(), "owner", "repo", "feature", "main")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if found {
		t.Error("expected PR to not be found")
	}
}

func TestCreatePR(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/repos/owner/repo/pulls" {
			t.Errorf("path = %s, want /repos/owner/repo/pulls", r.URL.Path)
		}
		assertAuthHeader(t, r)
		w.WriteHeader(http.StatusCreated)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"number":   7,
			"html_url": "https://github.com/owner/repo/pull/7",
		})
	})

	client, _ := newTestClient(t, handler)
	url, number, err := client.CreatePR(context.Background(), "owner", "repo", PRRequest{
		Title: "Update deps",
		Body:  "Automated update",
		Head:  "feature",
		Base:  "main",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if number != 7 {
		t.Errorf("PR number = %d, want 7", number)
	}
	if url != "https://github.com/owner/repo/pull/7" {
		t.Errorf("PR URL = %q, want https://github.com/owner/repo/pull/7", url)
	}
}

func TestUpdatePR(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Errorf("method = %s, want PATCH", r.Method)
		}
		if r.URL.Path != "/repos/owner/repo/pulls/99" {
			t.Errorf("path = %s, want /repos/owner/repo/pulls/99", r.URL.Path)
		}
		assertAuthHeader(t, r)

		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decoding body: %v", err)
		}
		if body["title"] != "New Title" {
			t.Errorf("body title = %q, want %q", body["title"], "New Title")
		}
		if body["body"] != "New Body" {
			t.Errorf("body body = %q, want %q", body["body"], "New Body")
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{"number": 99})
	})

	client, _ := newTestClient(t, handler)
	err := client.UpdatePR(context.Background(), "owner", "repo", 99, PRRequest{
		Title: "New Title",
		Body:  "New Body",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAPIError(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		w.Write([]byte(`{"message":"Validation Failed"}`))
	})

	client, _ := newTestClient(t, handler)
	_, err := client.GetDefaultBranch(context.Background(), "owner", "repo")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if apiErr.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("status code = %d, want %d", apiErr.StatusCode, http.StatusUnprocessableEntity)
	}
}

func TestAuthHeader(t *testing.T) {
	methods := []struct {
		name    string
		handler func(client *Client) error
		path    string
		method  string
	}{
		{
			name: "GET request",
			handler: func(client *Client) error {
				_, err := client.GetDefaultBranch(context.Background(), "owner", "repo")
				return err
			},
			path:   "/repos/owner/repo",
			method: http.MethodGet,
		},
		{
			name: "POST request",
			handler: func(client *Client) error {
				return client.CreateBranch(context.Background(), "owner", "repo", "branch", "sha")
			},
			path:   "/repos/owner/repo/git/refs",
			method: http.MethodPost,
		},
		{
			name: "PATCH request",
			handler: func(client *Client) error {
				return client.UpdatePR(context.Background(), "owner", "repo", 1, PRRequest{})
			},
			path:   "/repos/owner/repo/pulls/1",
			method: http.MethodPatch,
		},
	}

	for _, tc := range methods {
		t.Run(tc.name, func(t *testing.T) {
			var gotAuth string
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotAuth = r.Header.Get("Authorization")
				w.WriteHeader(http.StatusCreated)
				w.Write([]byte(`{}`))
			})

			client, _ := newTestClient(t, handler)
			tc.handler(client) // ignore error — we only care about the header

			want := "Bearer " + testToken
			if gotAuth != want {
				t.Errorf("Authorization = %q, want %q", gotAuth, want)
			}
		})
	}
}
