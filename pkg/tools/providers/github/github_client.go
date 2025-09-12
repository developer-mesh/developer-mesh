package github

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/google/go-github/v74/github"
	"github.com/shurcooL/githubv4"
	"golang.org/x/oauth2"
)

// GitHubClients holds all client types for GitHub API access
type GitHubClients struct {
	REST    *github.Client
	GraphQL *githubv4.Client
	Raw     *RawClient // For raw file access
}

// RawClient handles raw file downloads
type RawClient struct {
	httpClient *http.Client
	baseURL    string
	token      string
}

// GetClientFn is a function that returns a REST client
type GetClientFn func(context.Context) (*github.Client, error)

// GetGQLClientFn is a function that returns a GraphQL client
type GetGQLClientFn func(context.Context) (*githubv4.Client, error)

// GetRawClientFn is a function that returns a raw client
type GetRawClientFn func(context.Context) (*RawClient, error)

// BearerAuthTransport implements http.RoundTripper for bearer token auth
type BearerAuthTransport struct {
	Transport http.RoundTripper
	Token     string
}

// RoundTrip implements the http.RoundTripper interface
func (t *BearerAuthTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req2 := cloneRequest(req)
	req2.Header.Set("Authorization", "Bearer "+t.Token)
	req2.Header.Set("Accept", "application/vnd.github.v3+json")

	transport := t.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}

	return transport.RoundTrip(req2)
}

// cloneRequest creates a shallow copy of the request
func cloneRequest(r *http.Request) *http.Request {
	r2 := new(http.Request)
	*r2 = *r
	r2.Header = make(http.Header, len(r.Header))
	for k, s := range r.Header {
		r2.Header[k] = append([]string(nil), s...)
	}
	return r2
}

// NewGitHubClients creates all GitHub client types with authentication
func NewGitHubClients(token string, host string) (*GitHubClients, error) {
	// Default to github.com if no host specified
	if host == "" {
		host = "github.com"
	}

	// Determine API URLs based on host
	var apiURL, graphQLURL string
	if host == "github.com" {
		apiURL = "https://api.github.com"
		graphQLURL = "https://api.github.com/graphql"
	} else {
		// GitHub Enterprise
		apiURL = fmt.Sprintf("https://%s/api/v3", host)
		graphQLURL = fmt.Sprintf("https://%s/api/graphql", host)
	}

	// Create OAuth2 client for REST API
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	httpClient := oauth2.NewClient(ctx, ts)

	// REST client setup
	restClient := github.NewClient(httpClient)
	restClient.UserAgent = "devops-mcp-github/1.0"
	if host != "github.com" {
		restClient.BaseURL.Host = host
		restClient.BaseURL.Path = "/api/v3/"
	}

	// GraphQL client setup with bearer auth transport
	gqlHTTPClient := &http.Client{
		Transport: &BearerAuthTransport{
			Transport: http.DefaultTransport,
			Token:     token,
		},
	}

	var gqlClient *githubv4.Client
	if host == "github.com" {
		gqlClient = githubv4.NewClient(gqlHTTPClient)
	} else {
		gqlClient = githubv4.NewEnterpriseClient(graphQLURL, gqlHTTPClient)
	}

	// Raw client for direct file access
	rawClient := &RawClient{
		httpClient: httpClient,
		baseURL:    apiURL,
		token:      token,
	}

	return &GitHubClients{
		REST:    restClient,
		GraphQL: gqlClient,
		Raw:     rawClient,
	}, nil
}

// DownloadRawFile downloads a file's raw content
func (c *RawClient) DownloadRawFile(ctx context.Context, owner, repo, path, ref string) ([]byte, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/contents/%s", c.baseURL, owner, repo, path)
	if ref != "" {
		url += "?ref=" + ref
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/vnd.github.v3.raw")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to download file: %s", resp.Status)
	}

	return io.ReadAll(resp.Body)
}

// GetRawURL returns the raw content URL for a file
func (c *RawClient) GetRawURL(owner, repo, path, ref string) string {
	if ref == "" {
		ref = "main"
	}
	return fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/%s", owner, repo, ref, path)
}
