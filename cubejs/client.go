package cubejs

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

// maxResponseSize is the maximum response body size (10 MB).
const maxResponseSize = 10 << 20

// Client is an HTTP client for the Cube.js REST API.
type Client struct {
	baseURL    string
	httpClient *http.Client
	token      string
}

// NewClient creates a new Cube.js API client with Bearer token authentication.
// An optional *http.Client can be provided to share transports across clients;
// if nil, a new http.Client is created with the given timeout.
func NewClient(baseURL, jwtToken string, timeout int, httpClient ...*http.Client) *Client {
	var hc *http.Client
	if len(httpClient) > 0 && httpClient[0] != nil {
		hc = httpClient[0]
	} else {
		hc = &http.Client{
			Timeout: time.Duration(timeout) * time.Second,
		}
	}
	return &Client{
		baseURL:    baseURL,
		httpClient: hc,
		token:      jwtToken,
	}
}

// Meta fetches cube metadata (measures, dimensions, segments) via GET /cubejs-api/v1/meta.
func (c *Client) Meta(ctx context.Context) (*MetaResponse, error) {
	var result MetaResponse
	if err := c.doGet(ctx, "/cubejs-api/v1/meta", &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// Load executes a query and returns data via POST /cubejs-api/v1/load.
func (c *Client) Load(ctx context.Context, q *Query) (*LoadResponse, error) {
	var result LoadResponse
	if err := c.doPost(ctx, "/cubejs-api/v1/load", map[string]any{"query": q}, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// DryRun validates a query without executing it via POST /cubejs-api/v1/dry-run.
func (c *Client) DryRun(ctx context.Context, q *Query) (*DryRunResponse, error) {
	var result DryRunResponse
	if err := c.doPost(ctx, "/cubejs-api/v1/dry-run", map[string]any{"query": q}, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// SQL returns the generated SQL for a query via POST /cubejs-api/v1/sql.
func (c *Client) SQL(ctx context.Context, q *Query) (*SQLResponse, error) {
	var result SQLResponse
	if err := c.doPost(ctx, "/cubejs-api/v1/sql", map[string]any{"query": q}, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// PreAggregations lists all pre-aggregation definitions via GET /cubejs-api/v1/pre-aggregations.
func (c *Client) PreAggregations(ctx context.Context) (*PreAggregationsResponse, error) {
	var result PreAggregationsResponse
	if err := c.doGet(ctx, "/cubejs-api/v1/pre-aggregations", &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// TriggerPreAggregationJobs triggers a pre-aggregation rebuild via POST /cubejs-api/v1/pre-aggregations/jobs.
func (c *Client) TriggerPreAggregationJobs(ctx context.Context, selector *PreAggregationJobSelector) (PreAggregationJobsResponse, error) {
	body := PreAggregationJobsRequest{
		Action:   "post",
		Selector: selector,
	}
	var result PreAggregationJobsResponse
	if err := c.doPost(ctx, "/cubejs-api/v1/pre-aggregations/jobs", body, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// GetPreAggregationJobStatus checks job status via POST /cubejs-api/v1/pre-aggregations/jobs with tokens.
func (c *Client) GetPreAggregationJobStatus(ctx context.Context, tokens []string) (PreAggregationJobsResponse, error) {
	body := PreAggregationJobsRequest{
		Action: "get",
		Tokens: tokens,
	}
	var result PreAggregationJobsResponse
	if err := c.doPost(ctx, "/cubejs-api/v1/pre-aggregations/jobs", body, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *Client) doGet(ctx context.Context, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	return c.doRequest(req, out)
}

func (c *Client) doPost(ctx context.Context, path string, body any, out any) error {
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("failed to marshal request body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	return c.doRequest(req, out)
}

func (c *Client) doRequest(req *http.Request, out any) error {
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		slog.Error("cubejs request failed", "status", resp.StatusCode, "body", string(respBody))
		return fmt.Errorf("cubejs request failed with status %d", resp.StatusCode)
	}

	if err := json.Unmarshal(respBody, out); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	// Cube.js can return HTTP 200 with {"error": "Continue wait"} when a query
	// is queued for pre-aggregation. Response types embed cubeError so we detect
	// this in a single unmarshal pass.
	if eh, ok := out.(interface{ CubeError() string }); ok && eh.CubeError() != "" {
		return fmt.Errorf("cubejs: %s", eh.CubeError())
	}

	return nil
}
