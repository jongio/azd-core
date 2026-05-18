// Package httpclient provides an HTTP client with auth, retry, and pagination support.
package httpclient

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"
)

// Version is the version string used in the User-Agent header.
// It can be overridden at build time or by the caller.
var Version = "0.0.0-dev"

// UserAgent is the default User-Agent header value.
// It can be overridden by the caller.
var UserAgent = ""

func defaultUserAgent() string {
	if UserAgent != "" {
		return UserAgent
	}
	return fmt.Sprintf("azd-core-httpclient/%s", Version)
}

// TokenProvider supplies OAuth bearer tokens for a given scope.
type TokenProvider interface {
	GetToken(ctx context.Context, scope string) (string, error)
}

// RequestOptions contains options for HTTP requests.
// Fields are grouped by concern for clarity.
type RequestOptions struct {
	// Core request fields
	Method  string
	URL     string
	Body    io.Reader
	Headers map[string]string
	Verbose bool
	Timeout time.Duration

	// Authentication
	Scope         string
	SkipAuth      bool
	TokenProvider TokenProvider

	// Retry behavior
	Retry int

	// Redirect handling
	FollowRedirects bool
	MaxRedirects    int
	Insecure        bool

	// Output control
	OutputFile      string
	Format          string
	Binary          bool
	MaxResponseSize int64

	// Pagination
	Paginate bool
}

// effectiveMaxRedirects returns the configured max redirects or the default (10).
func (o *RequestOptions) effectiveMaxRedirects() int {
	if o.MaxRedirects > 0 {
		return o.MaxRedirects
	}
	return 10
}

// effectiveMaxRetries returns the configured retry count or the default (3).
func (o *RequestOptions) effectiveMaxRetries() int {
	if o.Retry > 0 {
		return o.Retry
	}
	return 3
}

// effectiveMaxResponseSize returns the configured max response size or the default (100MB).
func (o *RequestOptions) effectiveMaxResponseSize() int64 {
	if o.MaxResponseSize > 0 {
		return o.MaxResponseSize
	}
	return 100 * 1024 * 1024
}

// Response contains HTTP response data
type Response struct {
	StatusCode int
	Status     string
	Headers    http.Header
	Body       []byte
	Duration   time.Duration
}

// Client wraps HTTP client functionality
type Client struct {
	httpClient    *http.Client
	tokenProvider TokenProvider
}

// NewClient creates a new HTTP client
func NewClient(tokenProvider TokenProvider, insecure bool, timeout time.Duration) *Client {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: insecure, //nolint:gosec // G402: InsecureSkipVerify is intentionally configurable
		},
		// Use proxy from environment variables (HTTP_PROXY, HTTPS_PROXY, NO_PROXY)
		Proxy: http.ProxyFromEnvironment,
	}

	return &Client{
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   timeout,
		},
		tokenProvider: tokenProvider,
	}
}

// Execute performs an HTTP request with the given options
func (c *Client) Execute(ctx context.Context, opts RequestOptions) (*Response, error) {
	startTime := time.Now()

	client := c.buildRedirectClient(&opts)

	req, err := c.buildRequest(ctx, &opts)
	if err != nil {
		return nil, err
	}

	if opts.Verbose {
		logVerboseRequest(req, opts.Method, opts.URL)
	}

	resp, err := c.executeWithRetry(ctx, client, req, &opts)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	response, err := c.buildResponse(resp, &opts, time.Since(startTime))
	if err != nil {
		return nil, err
	}

	if opts.Paginate && resp.StatusCode >= 200 && resp.StatusCode < 300 {
		paginatedBody, err := handlePagination(ctx, client, opts, response)
		if err != nil {
			if opts.Verbose {
				fmt.Fprintf(os.Stderr, "Warning: Pagination failed: %v\n", err)
			}
			return response, nil
		}
		if paginatedBody != nil {
			response.Body = paginatedBody
		}
	}

	return response, nil
}

// buildRedirectClient creates an HTTP client with the configured redirect policy.
func (c *Client) buildRedirectClient(opts *RequestOptions) *http.Client {
	maxRedirects := opts.effectiveMaxRedirects()
	return &http.Client{
		Transport: c.httpClient.Transport,
		Timeout:   c.httpClient.Timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if !opts.FollowRedirects {
				return http.ErrUseLastResponse
			}
			if len(via) >= maxRedirects {
				return fmt.Errorf("stopped after %d redirects", maxRedirects)
			}
			return nil
		},
	}
}

// buildRequest creates the HTTP request with headers and authentication.
func (c *Client) buildRequest(ctx context.Context, opts *RequestOptions) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, opts.Method, opts.URL, opts.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	for key, value := range opts.Headers {
		req.Header.Set(key, value)
	}

	if !opts.SkipAuth && opts.Scope != "" && c.tokenProvider != nil {
		token, err := c.tokenProvider.GetToken(ctx, opts.Scope)
		if err != nil {
			return nil, fmt.Errorf("failed to get authentication token: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+token)
	}

	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", defaultUserAgent())
	}

	return req, nil
}

// logVerboseRequest logs request details to stderr in verbose mode.
func logVerboseRequest(req *http.Request, method, rawURL string) {
	fmt.Fprintf(os.Stderr, "> %s %s\n", method, RedactURL(rawURL))
	for key, values := range req.Header {
		for _, value := range values {
			fmt.Fprintf(os.Stderr, "> %s: %s\n", key, RedactSensitiveHeader(key, value))
		}
	}
	fmt.Fprintf(os.Stderr, "> \n")
}

// executeWithRetry runs the request with exponential backoff retry logic.
func (c *Client) executeWithRetry(ctx context.Context, client *http.Client, req *http.Request, opts *RequestOptions) (*http.Response, error) {
	maxRetries := opts.effectiveMaxRetries()

	// Buffer body for retry support
	var bodyBytes []byte
	bodyReader := opts.Body
	if opts.Body != nil && maxRetries > 0 {
		bodyReader, bodyBytes = bufferBodyForRetry(opts.Body)
		if br, ok := bodyReader.(*bytes.Reader); ok {
			_, _ = br.Seek(0, io.SeekStart)
			req.Body = io.NopCloser(br)
			req.ContentLength = int64(len(bodyBytes))
			req.GetBody = func() (io.ReadCloser, error) {
				_, _ = br.Seek(0, io.SeekStart)
				return io.NopCloser(br), nil
			}
		}
	}

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(1<<uint(attempt-1)) * time.Second //nolint:gosec // G115: safe, attempt is small
			select {
			case <-ctx.Done():
				return nil, fmt.Errorf("request canceled: %w", ctx.Err())
			case <-time.After(backoff):
			}
			resetRequestBody(req, bodyReader, bodyBytes)
		}

		resp, err := client.Do(req)
		if err == nil {
			if resp.StatusCode >= 500 && resp.StatusCode < 600 && attempt < maxRetries {
				_ = resp.Body.Close()
				continue
			}
			return resp, nil
		}

		lastErr = err
		if !isRetryableError(err) {
			return nil, fmt.Errorf("request failed: %w", err)
		}

		if attempt == maxRetries {
			return nil, fmt.Errorf("request failed after %d retries: %w", maxRetries, err)
		}
	}

	return nil, fmt.Errorf("request failed: %w", lastErr)
}

// bufferBodyForRetry reads the request body into memory for retry support.
// Returns a seekable reader and the buffered bytes (nil if body is too large).
func bufferBodyForRetry(body io.Reader) (io.Reader, []byte) {
	const maxBodySizeForRetry = 10 * 1024 * 1024
	limitedReader := io.LimitReader(body, maxBodySizeForRetry+1)
	bodyBytes, err := io.ReadAll(limitedReader)
	if err == nil && int64(len(bodyBytes)) <= maxBodySizeForRetry {
		return bytes.NewReader(bodyBytes), bodyBytes
	}
	if seeker, ok := body.(io.Seeker); ok {
		if _, seekErr := seeker.Seek(0, io.SeekStart); seekErr == nil {
			return body, nil
		}
	}
	return body, nil
}

// resetRequestBody resets the request body for a retry attempt.
func resetRequestBody(req *http.Request, bodyReader io.Reader, bodyBytes []byte) {
	if bodyReader == nil {
		return
	}
	if br, ok := bodyReader.(*bytes.Reader); ok {
		_, _ = br.Seek(0, io.SeekStart)
		req.Body = io.NopCloser(br)
		req.GetBody = func() (io.ReadCloser, error) {
			_, _ = br.Seek(0, io.SeekStart)
			return io.NopCloser(br), nil
		}
	} else if seekerReader, ok := bodyReader.(interface {
		io.Reader
		io.Seeker
	}); ok {
		_, _ = seekerReader.Seek(0, io.SeekStart)
		req.Body = io.NopCloser(seekerReader)
		req.GetBody = func() (io.ReadCloser, error) {
			_, _ = seekerReader.Seek(0, io.SeekStart)
			return io.NopCloser(seekerReader), nil
		}
	} else {
		req.Body = io.NopCloser(bodyReader)
	}
	_ = bodyBytes // suppress unused warning in non-bytes path
}

// buildResponse reads the response body and constructs the Response object.
func (c *Client) buildResponse(resp *http.Response, opts *RequestOptions, duration time.Duration) (*Response, error) {
	maxSize := opts.effectiveMaxResponseSize()
	limitedReader := io.LimitReader(resp.Body, maxSize)
	responseBody, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if int64(len(responseBody)) >= maxSize {
		return nil, fmt.Errorf("response body exceeds maximum size of %d bytes", maxSize)
	}

	return &Response{
		StatusCode: resp.StatusCode,
		Status:     resp.Status,
		Headers:    resp.Header,
		Body:       responseBody,
		Duration:   duration,
	}, nil
}

// ShouldSkipAuth determines if authentication should be skipped
func ShouldSkipAuth(url string, headers map[string]string, skipAuth bool) bool {
	// Explicit skip flag
	if skipAuth {
		return true
	}

	// Check if Authorization header already present
	for key := range headers {
		if strings.EqualFold(key, "authorization") {
			return true
		}
	}

	// Check if URL is HTTP (not HTTPS)
	if strings.HasPrefix(strings.ToLower(url), "http://") {
		return true
	}

	return false
}

// isRetryableError determines if an error is retryable
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	errStr := strings.ToLower(err.Error())

	retryablePatterns := []string{
		"timeout",
		"connection refused",
		"connection reset",
		"no such host",
		"network is unreachable",
		"temporary failure",
		"i/o timeout",
		"context deadline exceeded",
	}

	for _, pattern := range retryablePatterns {
		if strings.Contains(errStr, pattern) {
			return true
		}
	}

	return false
}

// DetectContentType attempts to determine if content is binary
func DetectContentType(body []byte, contentType string) bool {
	binaryTypes := []string{
		"application/octet-stream",
		"application/pdf",
		"image/",
		"video/",
		"audio/",
	}

	for _, binType := range binaryTypes {
		if strings.Contains(strings.ToLower(contentType), binType) {
			return true
		}
	}

	if len(body) > 0 {
		checkLen := 512
		if len(body) < checkLen {
			checkLen = len(body)
		}
		if bytes.ContainsRune(body[:checkLen], 0) {
			return true
		}
	}

	return false
}

// reLinkNext matches Link header with rel=next (case-insensitive on rel part only).
var reLinkNext = regexp.MustCompile(`(?i)<([^>]+)>;\s*rel=["']?next["']?`)

// parseLinkHeader parses the Link header and extracts the next URL.
// The regex is compiled once at package level for performance and matches
// the rel= attribute case-insensitively without lowercasing the URL.
func parseLinkHeader(linkHeader string) (string, bool) {
	if linkHeader == "" {
		return "", false
	}

	matches := reLinkNext.FindStringSubmatch(linkHeader)
	if len(matches) > 1 {
		return matches[1], true
	}

	return "", false
}

// extractNextLinkFromBody extracts nextLink from JSON response body (Azure API format)
func extractNextLinkFromBody(body []byte) (string, bool) {
	var data map[string]any
	if err := json.Unmarshal(body, &data); err != nil {
		return "", false
	}

	if nextLink, ok := data["nextLink"].(string); ok && nextLink != "" {
		return nextLink, true
	}

	if nextLink, ok := data["@odata.nextLink"].(string); ok && nextLink != "" {
		return nextLink, true
	}

	if next, ok := data["@odata.next"].(string); ok && next != "" {
		return next, true
	}

	return "", false
}

// handlePagination handles pagination by following next links.
// It enforces same-origin checks to prevent SSRF via server-controlled nextLink URLs.
func handlePagination(ctx context.Context, client *http.Client, opts RequestOptions, firstResponse *Response) ([]byte, error) {
	var allResults []any
	var currentBody = firstResponse.Body
	var nextURL string
	var hasMore = true

	// Parse original URL to enforce same-origin on pagination links
	originalURL, err := url.Parse(opts.URL)
	if err != nil {
		//nolint:nilerr // intentionally returning body despite read error
		return currentBody, nil
	}

	// Default max response size for pagination reads (same as Execute default)
	maxResponseSize := opts.MaxResponseSize
	if maxResponseSize <= 0 {
		maxResponseSize = 100 * 1024 * 1024 // Default 100MB limit
	}

	// Determine the token provider: prefer client-level, fall back to opts
	tokenProvider := opts.TokenProvider

	var firstData map[string]any
	if err := json.Unmarshal(currentBody, &firstData); err != nil {
		//nolint:nilerr // intentionally returning body despite read error
		return currentBody, nil
	}

	if valueArray, ok := firstData["value"].([]any); ok {
		allResults = append(allResults, valueArray...)
	} else {
		allResults = append(allResults, firstData)
	}

	if next, ok := extractNextLinkFromBody(currentBody); ok {
		nextURL = next
		hasMore = true
	} else if linkHeader := firstResponse.Headers.Get("Link"); linkHeader != "" {
		if next, ok := parseLinkHeader(linkHeader); ok {
			nextURL = next
			hasMore = true
		}
	}

	maxPages := 1000
	pageCount := 0

	for hasMore && nextURL != "" && pageCount < maxPages {
		pageCount++

		baseURL, err := url.Parse(opts.URL)
		if err != nil {
			break
		}
		nextURLParsed, err := url.Parse(nextURL)
		if err != nil {
			break
		}
		resolvedURL := baseURL.ResolveReference(nextURLParsed)

		// SECURITY: Enforce same-origin to prevent SSRF via server-controlled nextLink.
		// An attacker could inject a cross-origin URL to exfiltrate the bearer token.
		if resolvedURL.Scheme != originalURL.Scheme || resolvedURL.Host != originalURL.Host {
			break
		}

		resolvedURLStr := resolvedURL.String()

		if opts.Verbose {
			fmt.Fprintf(os.Stderr, "> Following pagination link: %s\n", RedactURL(resolvedURLStr))
		}

		req, err := http.NewRequestWithContext(ctx, opts.Method, resolvedURLStr, nil)
		if err != nil {
			break
		}

		for key, value := range opts.Headers {
			req.Header.Set(key, value)
		}

		if !opts.SkipAuth && opts.Scope != "" && tokenProvider != nil {
			token, err := tokenProvider.GetToken(ctx, opts.Scope)
			if err != nil {
				break
			}
			req.Header.Set("Authorization", "Bearer "+token)
		}

		if req.Header.Get("User-Agent") == "" {
			req.Header.Set("User-Agent", defaultUserAgent())
		}

		resp, err := client.Do(req)
		if err != nil {
			break
		}

		limitedReader := io.LimitReader(resp.Body, maxResponseSize)
		body, err := io.ReadAll(limitedReader)
		_ = resp.Body.Close()

		if err != nil {
			break
		}

		var pageData map[string]any
		if err := json.Unmarshal(body, &pageData); err != nil {
			break
		}

		if valueArray, ok := pageData["value"].([]any); ok {
			allResults = append(allResults, valueArray...)
		}

		nextURL = ""
		if next, ok := extractNextLinkFromBody(body); ok {
			nextURL = next
		} else if linkHeader := resp.Header.Get("Link"); linkHeader != "" {
			if next, ok := parseLinkHeader(linkHeader); ok {
				nextURL = next
			}
		}

		hasMore = (nextURL != "")
	}

	if len(allResults) > 0 {
		combined := map[string]any{
			"value": allResults,
		}

		for key, value := range firstData {
			if key != "value" && key != "nextLink" && key != "@odata.nextLink" && key != "@odata.next" {
				combined[key] = value
			}
		}

		delete(combined, "nextLink")
		delete(combined, "@odata.nextLink")
		delete(combined, "@odata.next")

		combinedJSON, err := json.Marshal(combined)
		if err != nil {
			return currentBody, err
		}

		return combinedJSON, nil
	}

	return currentBody, nil
}
