package cliapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	graphQLURL string
	config     Config
	httpClient *http.Client
}

type graphQLRequest struct {
	Query     string      `json:"query"`
	Variables interface{} `json:"variables,omitempty"`
}

type graphQLResponse[T any] struct {
	Data   T              `json:"data"`
	Errors []graphQLError `json:"errors"`
}

type graphQLError struct {
	Message string `json:"message"`
}

type AttachmentUploadParams struct {
	WorkspaceID string
	CaseID      string
	Description string
	Filename    string
	ContentType string
	Reader      io.Reader
}

type AttachmentUploadResult struct {
	ID          string `json:"id"`
	WorkspaceID string `json:"workspaceID"`
	CaseID      string `json:"caseID"`
	Filename    string `json:"filename"`
	ContentType string `json:"contentType"`
	Size        int64  `json:"size"`
	Status      string `json:"status"`
	Description string `json:"description"`
	Source      string `json:"source"`
}

func NewClient(cfg Config) *Client {
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}

	return &Client{
		graphQLURL: cfg.GraphQLURL,
		config:     cfg,
		httpClient: httpClient,
	}
}

func (c *Client) Query(ctx context.Context, query string, variables interface{}, out interface{}) error {
	body, err := json.Marshal(graphQLRequest{
		Query:     query,
		Variables: variables,
	})
	if err != nil {
		return fmt.Errorf("marshal graphql request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.graphQLURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	c.config.ApplyAuth(req)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("perform graphql request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read graphql response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("graphql request failed: status %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	envelope := graphQLResponse[json.RawMessage]{}
	if err := json.Unmarshal(respBody, &envelope); err != nil {
		return fmt.Errorf("decode graphql response: %w", err)
	}
	if len(envelope.Errors) > 0 {
		messages := make([]string, 0, len(envelope.Errors))
		for _, item := range envelope.Errors {
			if strings.TrimSpace(item.Message) != "" {
				messages = append(messages, strings.TrimSpace(item.Message))
			}
		}
		if len(messages) == 0 {
			return fmt.Errorf("graphql request failed")
		}
		return fmt.Errorf("graphql request failed: %s", strings.Join(messages, "; "))
	}
	if out == nil {
		return nil
	}
	if len(envelope.Data) == 0 {
		return fmt.Errorf("graphql response missing data")
	}
	if err := json.Unmarshal(envelope.Data, out); err != nil {
		return fmt.Errorf("decode graphql data: %w", err)
	}
	return nil
}

func (c *Client) UploadAttachment(ctx context.Context, params AttachmentUploadParams) (*AttachmentUploadResult, error) {
	if strings.TrimSpace(params.WorkspaceID) == "" {
		return nil, fmt.Errorf("workspace ID is required")
	}
	if strings.TrimSpace(params.Filename) == "" {
		return nil, fmt.Errorf("filename is required")
	}
	if params.Reader == nil {
		return nil, fmt.Errorf("reader is required")
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("workspace_id", params.WorkspaceID); err != nil {
		return nil, fmt.Errorf("write workspace field: %w", err)
	}
	if strings.TrimSpace(params.CaseID) != "" {
		if err := writer.WriteField("case_id", params.CaseID); err != nil {
			return nil, fmt.Errorf("write case field: %w", err)
		}
	}
	if strings.TrimSpace(params.Description) != "" {
		if err := writer.WriteField("description", params.Description); err != nil {
			return nil, fmt.Errorf("write description field: %w", err)
		}
	}
	if strings.TrimSpace(params.ContentType) != "" {
		if err := writer.WriteField("content_type", params.ContentType); err != nil {
			return nil, fmt.Errorf("write content type field: %w", err)
		}
	}

	part, err := writer.CreateFormFile("file", params.Filename)
	if err != nil {
		return nil, fmt.Errorf("create multipart file field: %w", err)
	}
	if _, err := io.Copy(part, params.Reader); err != nil {
		return nil, fmt.Errorf("copy file content: %w", err)
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("close multipart writer: %w", err)
	}

	uploadURL := strings.TrimRight(c.config.APIBaseURL, "/") + "/attachments"
	if c.config.AuthMode == AuthModeSession {
		uploadURL = strings.TrimRight(c.config.AdminBaseURL, "/") + "/actions/attachments"
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, uploadURL, &body)
	if err != nil {
		return nil, fmt.Errorf("build upload request: %w", err)
	}
	c.config.ApplyAuth(req)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("perform upload request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read upload response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("upload request failed: status %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	var result AttachmentUploadResult
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("decode upload response: %w", err)
	}
	return &result, nil
}
