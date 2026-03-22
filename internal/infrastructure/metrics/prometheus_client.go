package metrics

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// PrometheusClient queries the Prometheus HTTP API for metrics
type PrometheusClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewPrometheusClient creates a new Prometheus client
func NewPrometheusClient(prometheusURL string) *PrometheusClient {
	if prometheusURL == "" {
		prometheusURL = "http://prometheus:9090"
	}
	return &PrometheusClient{
		baseURL: prometheusURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// QueryResult represents a Prometheus query result
type QueryResult struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Metric map[string]string `json:"metric"`
			Value  []interface{}     `json:"value"`
		} `json:"result"`
	} `json:"data"`
}

// Query executes a PromQL query and returns the result
func (c *PrometheusClient) Query(ctx context.Context, query string) (*QueryResult, error) {
	u, err := url.Parse(c.baseURL + "/api/v1/query")
	if err != nil {
		return nil, fmt.Errorf("failed to parse URL: %w", err)
	}

	q := u.Query()
	q.Set("query", query)
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("prometheus returned status %d (failed to read body: %v)", resp.StatusCode, err)
		}
		return nil, fmt.Errorf("prometheus returned status %d: %s", resp.StatusCode, string(body))
	}

	var result QueryResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if result.Status != "success" {
		return nil, fmt.Errorf("prometheus query failed with status: %s", result.Status)
	}

	return &result, nil
}

// GetScalarValue extracts a single scalar value from a query result
func (c *PrometheusClient) GetScalarValue(result *QueryResult) (float64, error) {
	if len(result.Data.Result) == 0 {
		return 0, nil // No data, return 0
	}

	if len(result.Data.Result[0].Value) < 2 {
		return 0, fmt.Errorf("invalid result format")
	}

	// Value is [timestamp, "value"]
	valueStr, ok := result.Data.Result[0].Value[1].(string)
	if !ok {
		return 0, fmt.Errorf("value is not a string")
	}

	value, err := strconv.ParseFloat(valueStr, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse value: %w", err)
	}

	return value, nil
}

// GetSumValue sums all values from a query result (for summing across labels)
func (c *PrometheusClient) GetSumValue(result *QueryResult) float64 {
	var sum float64
	for _, r := range result.Data.Result {
		if len(r.Value) >= 2 {
			if valueStr, ok := r.Value[1].(string); ok {
				if value, err := strconv.ParseFloat(valueStr, 64); err == nil {
					sum += value
				}
			}
		}
	}
	return sum
}

// GetLabelValues extracts values grouped by a specific label
func (c *PrometheusClient) GetLabelValues(result *QueryResult, labelName string) map[string]float64 {
	values := make(map[string]float64)
	for _, r := range result.Data.Result {
		labelValue, exists := r.Metric[labelName]
		if !exists {
			continue
		}

		if len(r.Value) >= 2 {
			if valueStr, ok := r.Value[1].(string); ok {
				if value, err := strconv.ParseFloat(valueStr, 64); err == nil {
					values[labelValue] = value
				}
			}
		}
	}
	return values
}

// Analytics-specific helper methods

// GetErrorRate returns errors per minute over the specified duration
func (c *PrometheusClient) GetErrorRate(ctx context.Context, duration string) (float64, error) {
	query := fmt.Sprintf("rate(mbr_errors_ingested_total[%s]) * 60", duration)
	result, err := c.Query(ctx, query)
	if err != nil {
		return 0, err
	}
	return c.GetSumValue(result), nil
}

// GetTotalErrors returns total errors ingested
func (c *PrometheusClient) GetTotalErrors(ctx context.Context) (float64, error) {
	query := "sum(mbr_errors_ingested_total)"
	result, err := c.Query(ctx, query)
	if err != nil {
		return 0, err
	}
	return c.GetScalarValue(result)
}

// GetActiveWorkspaces returns the current number of active workspaces
func (c *PrometheusClient) GetActiveWorkspaces(ctx context.Context) (float64, error) {
	query := "mbr_active_workspaces"
	result, err := c.Query(ctx, query)
	if err != nil {
		return 0, err
	}
	return c.GetScalarValue(result)
}

// GetErrorsByWorkspace returns error counts grouped by workspace
func (c *PrometheusClient) GetErrorsByWorkspace(ctx context.Context) (map[string]float64, error) {
	query := "sum by (workspace) (mbr_errors_ingested_total)"
	result, err := c.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	return c.GetLabelValues(result, "workspace"), nil
}

// GetCasesByWorkspace returns case counts grouped by workspace
func (c *PrometheusClient) GetCasesByWorkspace(ctx context.Context) (map[string]float64, error) {
	query := "sum by (workspace) (mbr_cases_created_total)"
	result, err := c.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	return c.GetLabelValues(result, "workspace"), nil
}

// GetErrorsBySeverity returns error counts grouped by severity
func (c *PrometheusClient) GetErrorsBySeverity(ctx context.Context) (map[string]float64, error) {
	query := "sum by (severity) (mbr_cases_created_total)"
	result, err := c.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	return c.GetLabelValues(result, "severity"), nil
}

// GetRecentErrors returns error count over the last duration
func (c *PrometheusClient) GetRecentErrors(ctx context.Context, duration string) (float64, error) {
	query := fmt.Sprintf("sum(increase(mbr_errors_ingested_total[%s]))", duration)
	result, err := c.Query(ctx, query)
	if err != nil {
		return 0, err
	}
	return c.GetScalarValue(result)
}
