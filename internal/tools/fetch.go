package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/arawak/mcp-fetch-go/internal/fetch"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type FetchTool struct {
	Name        string
	Description string
}

type FetchArgs struct {
	URL        string `json:"url" jsonschema:"The URL to fetch"`
	MaxBytes   *int   `json:"maxBytes,omitempty" jsonschema:"Maximum number of bytes to return"`
	StartBytes *int   `json:"startBytes,omitempty" jsonschema:"Number of bytes to skip before reading"`
	Raw        *bool  `json:"raw,omitempty" jsonschema:"Return raw content without Markdown conversion"`
	Timeout    *int   `json:"timeout,omitempty" jsonschema:"Timeout in seconds"`
}

func (t *FetchTool) Run(ctx context.Context, req *mcp.CallToolRequest, args FetchArgs) (*mcp.CallToolResult, interface{}, error) {
	if args.URL == "" {
		return nil, nil, fmt.Errorf("missing required input: url")
	}
	maxBytes := 30000
	if args.MaxBytes != nil {
		maxBytes = *args.MaxBytes
	}
	startBytes := 0
	if args.StartBytes != nil {
		startBytes = *args.StartBytes
	}
	raw := false
	if args.Raw != nil {
		raw = *args.Raw
	}
	timeout := 30
	if args.Timeout != nil {
		timeout = *args.Timeout
	}
	if maxBytes < 0 || startBytes < 0 {
		return nil, nil, fmt.Errorf("invalid byte range")
	}

	fetchResult, err := fetch.FetchURL(ctx, args.URL, maxBytes, startBytes, raw, timeout)
	if err != nil {
		return nil, nil, err
	}

	content := fetchResult.Content
	if fetchResult.FinalURL != "" && fetchResult.FinalURL != args.URL {
		content = fmt.Sprintf("[Note: Redirected to %s]\n\n%s", fetchResult.FinalURL, content)
	}

	// Build metadata JSON.
	metadata := map[string]interface{}{
		"url":           fetchResult.URL,
		"finalURL":      fetchResult.FinalURL,
		"statusCode":    fetchResult.StatusCode,
		"contentType":   fetchResult.ContentType,
		"contentBytes":  fetchResult.ContentBytes,
		"returnedBytes": fetchResult.ReturnedBytes,
		"truncated":     fetchResult.Truncated,
		"fetchedAt":     fetchResult.FetchedAt,
	}
	if fetchResult.NextStartBytes != nil {
		metadata["nextStartBytes"] = *fetchResult.NextStartBytes
	}
	// Only include finalURL if it differs from the requested URL.
	if fetchResult.FinalURL == "" || fetchResult.FinalURL == args.URL {
		delete(metadata, "finalURL")
	}

	metadataJSON, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal metadata: %w", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: content},
			&mcp.TextContent{Text: "---\n" + string(metadataJSON)},
		},
	}, nil, nil
}

func NewFetchTool() *FetchTool {
	return &FetchTool{
		Name:        "fetch",
		Description: "Fetch a URL and return content",
	}
}
