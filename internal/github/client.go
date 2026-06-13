package github

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type Client struct {
	httpClient *http.Client
	baseURL    string
	token      string
}

func NewClient(httpClient *http.Client, token string) *Client {
	return &Client{
		httpClient: httpClient,
		baseURL:    "https://api.github.com",
		token:      token,
	}
}

// NewClientWithBaseURL is used for testing with httptest servers
func NewClientWithBaseURL(httpClient *http.Client, token, baseURL string) *Client {
	return &Client{
		httpClient: httpClient,
		baseURL:    baseURL,
		token:      token,
	}
}

func (c *Client) GetDefaultBranch(ctx context.Context, owner, repo string) (string, error) {
	path := fmt.Sprintf("/repos/%s/%s", owner, repo)
	var result struct {
		DefaultBranch string `json:"default_branch"`
	}
	if err := c.get(ctx, path, &result); err != nil {
		return "", err
	}
	return result.DefaultBranch, nil
}

func (c *Client) GetRef(ctx context.Context, owner, repo, ref string) (string, error) {
	path := fmt.Sprintf("/repos/%s/%s/git/ref/%s", owner, repo, ref)
	var result struct {
		Object struct {
			SHA string `json:"sha"`
		} `json:"object"`
	}
	if err := c.get(ctx, path, &result); err != nil {
		return "", err
	}
	return result.Object.SHA, nil
}

func (c *Client) BranchExists(ctx context.Context, owner, repo, branch string) (bool, error) {
	path := fmt.Sprintf("/repos/%s/%s/git/ref/heads/%s", owner, repo, branch)
	req, err := c.newRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return false, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	if resp.StatusCode == http.StatusNotFound {
		return false, nil
	}
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	return true, nil
}

func (c *Client) CreateBranch(ctx context.Context, owner, repo, branch, fromSHA string) error {
	path := fmt.Sprintf("/repos/%s/%s/git/refs", owner, repo)
	body := map[string]string{
		"ref": "refs/heads/" + branch,
		"sha": fromSHA,
	}
	return c.post(ctx, path, body, nil)
}

func (c *Client) UpdateRef(ctx context.Context, owner, repo, ref, sha string) error {
	path := fmt.Sprintf("/repos/%s/%s/git/refs/%s", owner, repo, ref)
	body := map[string]interface{}{
		"sha":   sha,
		"force": true,
	}
	return c.patch(ctx, path, body, nil)
}

func (c *Client) CreateBlob(ctx context.Context, owner, repo string, content []byte) (string, error) {
	path := fmt.Sprintf("/repos/%s/%s/git/blobs", owner, repo)
	body := map[string]string{
		"content":  base64.StdEncoding.EncodeToString(content),
		"encoding": "base64",
	}
	var result struct {
		SHA string `json:"sha"`
	}
	if err := c.post(ctx, path, body, &result); err != nil {
		return "", err
	}
	return result.SHA, nil
}

func (c *Client) CreateTree(ctx context.Context, owner, repo, baseTree string, entries []TreeEntry) (string, error) {
	path := fmt.Sprintf("/repos/%s/%s/git/trees", owner, repo)
	body := map[string]interface{}{
		"base_tree": baseTree,
		"tree":      entries,
	}
	var result struct {
		SHA string `json:"sha"`
	}
	if err := c.post(ctx, path, body, &result); err != nil {
		return "", err
	}
	return result.SHA, nil
}

type TreeEntry struct {
	Path string `json:"path"`
	Mode string `json:"mode"`
	Type string `json:"type"`
	SHA  string `json:"sha"`
}

func (c *Client) CreateCommit(ctx context.Context, owner, repo, message, treeSHA string, parents []string) (string, error) {
	path := fmt.Sprintf("/repos/%s/%s/git/commits", owner, repo)
	body := map[string]interface{}{
		"message": message,
		"tree":    treeSHA,
		"parents": parents,
	}
	var result struct {
		SHA string `json:"sha"`
	}
	if err := c.post(ctx, path, body, &result); err != nil {
		return "", err
	}
	return result.SHA, nil
}

// CommitFiles creates blobs, a tree, and a commit for the given files on a branch.
func (c *Client) CommitFiles(ctx context.Context, owner, repo, branch, message string, files map[string][]byte) (string, error) {
	// Get current branch SHA
	branchSHA, err := c.GetRef(ctx, owner, repo, "heads/"+branch)
	if err != nil {
		return "", fmt.Errorf("getting branch ref: %w", err)
	}

	// Get the commit to find the tree
	var commit struct {
		Tree struct {
			SHA string `json:"sha"`
		} `json:"tree"`
	}
	commitPath := fmt.Sprintf("/repos/%s/%s/git/commits/%s", owner, repo, branchSHA)
	if err := c.get(ctx, commitPath, &commit); err != nil {
		return "", fmt.Errorf("getting commit: %w", err)
	}

	// Create blobs and tree entries
	var entries []TreeEntry
	for path, content := range files {
		blobSHA, err := c.CreateBlob(ctx, owner, repo, content)
		if err != nil {
			return "", fmt.Errorf("creating blob for %s: %w", path, err)
		}
		entries = append(entries, TreeEntry{
			Path: path,
			Mode: "100644",
			Type: "blob",
			SHA:  blobSHA,
		})
	}

	// Create tree
	treeSHA, err := c.CreateTree(ctx, owner, repo, commit.Tree.SHA, entries)
	if err != nil {
		return "", fmt.Errorf("creating tree: %w", err)
	}

	// Create commit
	newCommitSHA, err := c.CreateCommit(ctx, owner, repo, message, treeSHA, []string{branchSHA})
	if err != nil {
		return "", fmt.Errorf("creating commit: %w", err)
	}

	// Update branch ref
	if err := c.UpdateRef(ctx, owner, repo, "heads/"+branch, newCommitSHA); err != nil {
		return "", fmt.Errorf("updating ref: %w", err)
	}

	return newCommitSHA, nil
}

func (c *Client) FindOpenPR(ctx context.Context, owner, repo, head, base string) (int, bool, error) {
	path := fmt.Sprintf("/repos/%s/%s/pulls?state=open&head=%s:%s&base=%s", owner, repo, owner, head, base)
	var prs []PullRequest
	if err := c.get(ctx, path, &prs); err != nil {
		return 0, false, err
	}
	if len(prs) == 0 {
		return 0, false, nil
	}
	return prs[0].Number, true, nil
}

func (c *Client) CreatePR(ctx context.Context, owner, repo string, pr PRRequest) (string, int, error) {
	path := fmt.Sprintf("/repos/%s/%s/pulls", owner, repo)
	body := map[string]interface{}{
		"title": pr.Title,
		"body":  pr.Body,
		"head":  pr.Head,
		"base":  pr.Base,
		"draft": pr.Draft,
	}
	var result struct {
		Number  int    `json:"number"`
		HTMLURL string `json:"html_url"`
	}
	if err := c.post(ctx, path, body, &result); err != nil {
		return "", 0, err
	}
	return result.HTMLURL, result.Number, nil
}

func (c *Client) UpdatePR(ctx context.Context, owner, repo string, prNumber int, pr PRRequest) error {
	path := fmt.Sprintf("/repos/%s/%s/pulls/%d", owner, repo, prNumber)
	body := map[string]interface{}{
		"title": pr.Title,
		"body":  pr.Body,
	}
	return c.patch(ctx, path, body, nil)
}

func (c *Client) AddLabels(ctx context.Context, owner, repo string, prNumber int, labels []string) error {
	if len(labels) == 0 {
		return nil
	}
	path := fmt.Sprintf("/repos/%s/%s/issues/%d/labels", owner, repo, prNumber)
	body := map[string]interface{}{
		"labels": labels,
	}
	return c.post(ctx, path, body, nil)
}

func (c *Client) newRequest(ctx context.Context, method, path string, body interface{}) (*http.Request, error) {
	url := c.baseURL + path

	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshaling request body: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/vnd.github+json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	return req, nil
}

func (c *Client) get(ctx context.Context, path string, result interface{}) error {
	req, err := c.newRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return err
	}
	return c.do(req, result)
}

func (c *Client) post(ctx context.Context, path string, body, result interface{}) error {
	req, err := c.newRequest(ctx, http.MethodPost, path, body)
	if err != nil {
		return err
	}
	return c.do(req, result)
}

func (c *Client) patch(ctx context.Context, path string, body, result interface{}) error {
	req, err := c.newRequest(ctx, http.MethodPatch, path, body)
	if err != nil {
		return err
	}
	return c.do(req, result)
}

func (c *Client) do(req *http.Request, result interface{}) error {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20)) // 10 MB limit
	if err != nil {
		return fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &APIError{
			StatusCode: resp.StatusCode,
			Body:       string(respBody),
		}
	}

	if result != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, result); err != nil {
			return fmt.Errorf("decoding response: %w", err)
		}
	}

	return nil
}

type APIError struct {
	StatusCode int
	Body       string
}

func (e *APIError) Error() string {
	body := e.Body
	if len(body) > 200 {
		body = body[:200] + "...(truncated)"
	}
	return fmt.Sprintf("GitHub API error (status %d): %s", e.StatusCode, body)
}
