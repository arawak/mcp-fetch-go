package tools_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/arawak/mcp-fetch-go/internal/tools"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func textContent(t *testing.T, result *mcp.CallToolResult) string {
	t.Helper()
	// We expect either 1 or 2 content items: markdown content and optional metadata JSON.
	if len(result.Content) == 0 {
		t.Fatalf("expected at least one content item, got none")
	}
	if len(result.Content) > 2 {
		t.Fatalf("expected at most 2 content items, got %d", len(result.Content))
	}
	tc, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("expected *mcp.TextContent, got %T", result.Content[0])
	}
	return tc.Text
}

func TestFetchToolRun(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("hello world"))
	}))
	defer server.Close()

	t.Setenv("MCP_FETCH_ALLOW_PRIVATE", "true")

	tool := tools.NewFetchTool()
	ctx := context.Background()
	maxBytes := 1024
	startBytes := 0
	raw := false
	timeout := 10
	args := tools.FetchArgs{
		URL:        server.URL,
		MaxBytes:   &maxBytes,
		StartBytes: &startBytes,
		Raw:        &raw,
		Timeout:    &timeout,
	}

	result, _, err := tool.Run(ctx, &mcp.CallToolRequest{}, args)
	if err != nil {
		t.Fatalf("fetch failed: %v", err)
	}
	text := textContent(t, result)
	if text != "hello world" {
		t.Fatalf("unexpected content: %q", text)
	}
}

func TestFetchToolByteWindow(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("0123456789"))
	}))
	defer server.Close()

	t.Setenv("MCP_FETCH_ALLOW_PRIVATE", "true")

	tool := tools.NewFetchTool()
	ctx := context.Background()
	maxBytes := 4
	startBytes := 3
	raw := true
	timeout := 10
	args := tools.FetchArgs{
		URL:        server.URL,
		MaxBytes:   &maxBytes,
		StartBytes: &startBytes,
		Raw:        &raw,
		Timeout:    &timeout,
	}

	result, _, err := tool.Run(ctx, &mcp.CallToolRequest{}, args)
	if err != nil {
		t.Fatalf("fetch failed: %v", err)
	}
	text := textContent(t, result)
	// Content contains the continuation header and the sliced body.
	// "3456" are the 4 bytes starting at offset 3; the body has no paragraph
	// breaks so snapping falls back to hard cut.
	if !strings.Contains(text, "3456") {
		t.Fatalf("expected sliced content %q in output, got %q", "3456", text)
	}
	if !strings.Contains(text, "Truncated") {
		t.Fatalf("expected truncation notice in output, got: %q", text)
	}
	if !strings.Contains(text, "Continued from byte 3") {
		t.Fatalf("expected continuation notice in output, got: %q", text)
	}
}

func TestFetchToolHTMLToMarkdown(t *testing.T) {
	body := `<!DOCTYPE html><html><head><title>T</title></head><body>
<nav><a href="/">Home</a></nav>
<main><h1>Hello</h1><p>World</p></main>
<footer>Footer</footer>
</body></html>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(body))
	}))
	defer server.Close()

	t.Setenv("MCP_FETCH_ALLOW_PRIVATE", "true")

	tool := tools.NewFetchTool()
	ctx := context.Background()
	raw := false
	timeout := 10
	args := tools.FetchArgs{
		URL:     server.URL,
		Raw:     &raw,
		Timeout: &timeout,
	}

	result, _, err := tool.Run(ctx, &mcp.CallToolRequest{}, args)
	if err != nil {
		t.Fatalf("fetch failed: %v", err)
	}
	text := textContent(t, result)

	if strings.Contains(text, "<nav>") || strings.Contains(text, "<footer>") {
		t.Fatalf("raw HTML leaked into output: %q", text)
	}
	if !strings.Contains(text, "Hello") {
		t.Fatalf("expected heading in output, got: %q", text)
	}
	if strings.Contains(text, "Home") {
		t.Fatalf("nav link should have been stripped, got: %q", text)
	}
	if strings.Contains(text, "Footer") {
		t.Fatalf("footer content should have been stripped, got: %q", text)
	}
}

func TestFetchToolDomainRestriction(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("blocked"))
	}))
	defer server.Close()

	t.Setenv("MCP_FETCH_ALLOW_PRIVATE", "false")

	tool := tools.NewFetchTool()
	ctx := context.Background()
	maxBytes := 128
	startBytes := 0
	raw := true
	timeout := 10
	args := tools.FetchArgs{
		URL:        server.URL,
		MaxBytes:   &maxBytes,
		StartBytes: &startBytes,
		Raw:        &raw,
		Timeout:    &timeout,
	}

	_, _, err := tool.Run(ctx, &mcp.CallToolRequest{}, args)
	if err == nil {
		t.Fatal("expected private address error")
	}
	if !strings.Contains(err.Error(), "private or non-global address") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFetchToolTruncationNotice(t *testing.T) {
	body := strings.Repeat("x", 5000)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte(body))
	}))
	defer server.Close()

	t.Setenv("MCP_FETCH_ALLOW_PRIVATE", "true")

	tool := tools.NewFetchTool()
	ctx := context.Background()
	maxBytes := 100
	startBytes := 0
	raw := true
	timeout := 10
	args := tools.FetchArgs{
		URL:        server.URL,
		MaxBytes:   &maxBytes,
		StartBytes: &startBytes,
		Raw:        &raw,
		Timeout:    &timeout,
	}

	result, _, err := tool.Run(ctx, &mcp.CallToolRequest{}, args)
	if err != nil {
		t.Fatalf("fetch failed: %v", err)
	}
	text := textContent(t, result)
	if !strings.Contains(text, "Truncated") {
		t.Errorf("expected truncation notice, got: %q", text)
	}
	if !strings.Contains(text, "Use startBytes=100 to continue") {
		t.Errorf("expected next startBytes hint, got: %q", text)
	}
}

func TestFetchToolMissingURL(t *testing.T) {
	tool := tools.NewFetchTool()
	ctx := context.Background()
	args := tools.FetchArgs{}

	_, _, err := tool.Run(ctx, &mcp.CallToolRequest{}, args)
	if err == nil {
		t.Fatal("expected error for missing URL")
	}
	if !strings.Contains(err.Error(), "url") {
		t.Errorf("expected url-related error, got: %v", err)
	}
}

func TestFetchToolUnsupportedContentType(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("\x89PNG"))
	}))
	defer server.Close()

	t.Setenv("MCP_FETCH_ALLOW_PRIVATE", "true")

	tool := tools.NewFetchTool()
	ctx := context.Background()
	raw := false
	args := tools.FetchArgs{URL: server.URL, Raw: &raw}

	_, _, err := tool.Run(ctx, &mcp.CallToolRequest{}, args)
	if err == nil {
		t.Fatal("expected error for unsupported content type")
	}
	if !strings.Contains(err.Error(), "unsupported content type") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestFetchToolRedirectNotice(t *testing.T) {
	// Destination server that returns actual content
	dest := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("destination content"))
	}))
	defer dest.Close()

	// Redirector server that redirects to destination
	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, dest.URL, http.StatusFound)
	}))
	defer redirector.Close()

	t.Setenv("MCP_FETCH_ALLOW_PRIVATE", "true")

	tool := tools.NewFetchTool()
	ctx := context.Background()
	maxBytes := 1024
	startBytes := 0
	raw := true
	timeout := 10
	args := tools.FetchArgs{
		URL:        redirector.URL,
		MaxBytes:   &maxBytes,
		StartBytes: &startBytes,
		Raw:        &raw,
		Timeout:    &timeout,
	}

	result, _, err := tool.Run(ctx, &mcp.CallToolRequest{}, args)
	if err != nil {
		t.Fatalf("fetch failed: %v", err)
	}
	text := textContent(t, result)

	// Should contain redirect notice
	if !strings.Contains(text, "[Note: Redirected to") {
		t.Errorf("expected redirect notice in output, got: %q", text)
	}
	// Should contain the destination URL in the notice
	if !strings.Contains(text, dest.URL) {
		t.Errorf("expected destination URL in redirect notice, got: %q", text)
	}
	// Should still contain the actual content
	if !strings.Contains(text, "destination content") {
		t.Errorf("expected destination content in output, got: %q", text)
	}
}

func TestFetchToolMetadataBlock(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("hello world"))
	}))
	defer server.Close()

	t.Setenv("MCP_FETCH_ALLOW_PRIVATE", "true")

	tool := tools.NewFetchTool()
	ctx := context.Background()
	maxBytes := 1024
	startBytes := 0
	raw := true
	timeout := 10
	args := tools.FetchArgs{
		URL:        server.URL,
		MaxBytes:   &maxBytes,
		StartBytes: &startBytes,
		Raw:        &raw,
		Timeout:    &timeout,
	}

	result, _, err := tool.Run(ctx, &mcp.CallToolRequest{}, args)
	if err != nil {
		t.Fatalf("fetch failed: %v", err)
	}

	// Should have two content blocks: markdown and metadata JSON.
	if len(result.Content) != 2 {
		t.Fatalf("expected 2 content blocks, got %d", len(result.Content))
	}

	// First block should be the markdown content.
	textContent, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("expected *mcp.TextContent, got %T", result.Content[0])
	}
	if textContent.Text != "hello world" {
		t.Errorf("expected content 'hello world', got: %q", textContent.Text)
	}

	// Second block should be the metadata JSON.
	metaContent, ok := result.Content[1].(*mcp.TextContent)
	if !ok {
		t.Fatalf("expected *mcp.TextContent for metadata, got %T", result.Content[1])
	}
	metaText := metaContent.Text
	if !strings.HasPrefix(metaText, "---\n") {
		t.Errorf("metadata should start with ---, got: %q", metaText)
	}
	if !strings.Contains(metaText, "\"url\"") {
		t.Errorf("metadata should contain url field, got: %q", metaText)
	}
	if !strings.Contains(metaText, "\"statusCode\": 200") {
		t.Errorf("metadata should contain statusCode field, got: %q", metaText)
	}
}

func TestFetchToolMetadataBlock_Truncated(t *testing.T) {
	body := strings.Repeat("x", 5000)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte(body))
	}))
	defer server.Close()

	t.Setenv("MCP_FETCH_ALLOW_PRIVATE", "true")

	tool := tools.NewFetchTool()
	ctx := context.Background()
	maxBytes := 100
	startBytes := 0
	raw := true
	timeout := 10
	args := tools.FetchArgs{
		URL:        server.URL,
		MaxBytes:   &maxBytes,
		StartBytes: &startBytes,
		Raw:        &raw,
		Timeout:    &timeout,
	}

	result, _, err := tool.Run(ctx, &mcp.CallToolRequest{}, args)
	if err != nil {
		t.Fatalf("fetch failed: %v", err)
	}

	// Should have two content blocks.
	if len(result.Content) != 2 {
		t.Fatalf("expected 2 content blocks, got %d", len(result.Content))
	}

	// Second block should contain metadata with truncated info.
	metaContent := result.Content[1].(*mcp.TextContent)
	metaText := metaContent.Text
	if !strings.Contains(metaText, "\"truncated\": true") {
		t.Errorf("metadata should contain truncated: true, got: %q", metaText)
	}
	if !strings.Contains(metaText, "\"nextStartBytes\"") {
		t.Errorf("metadata should contain nextStartBytes when truncated, got: %q", metaText)
	}
	if !strings.Contains(metaText, "\"contentBytes\": 5000") {
		t.Errorf("metadata should contain correct contentBytes, got: %q", metaText)
	}
}
