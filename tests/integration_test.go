package tools_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/arawak/mcp-fetch-go/internal/tools"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type SnapshotTest struct {
	URL         string `json:"url"`
	Fixture     string `json:"fixture"`
	Captured    string `json:"captured"`
	ContentType string `json:"contentType"`
	Tests       struct {
		Contains    []string `json:"contains"`
		NotContains []string `json:"notContains"`
		MinLength   int      `json:"minLength"`
	} `json:"tests"`
}

func loadSnapshots(t *testing.T) map[string]SnapshotTest {
	t.Helper()
	data, err := os.ReadFile("snapshots.json")
	if err != nil {
		t.Fatalf("failed to read snapshots.json: %v", err)
	}
	var snapshots map[string]SnapshotTest
	if err := json.Unmarshal(data, &snapshots); err != nil {
		t.Fatalf("failed to parse snapshots.json: %v", err)
	}
	return snapshots
}

func TestIntegration_Snapshots(t *testing.T) {
	snapshots := loadSnapshots(t)
	testsDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}

	for name, snap := range snapshots {
		snap := snap
		t.Run(name, func(t *testing.T) {

			fixturePath := filepath.Join(testsDir, snap.Fixture)
			htmlData, err := os.ReadFile(fixturePath)
			if err != nil {
				t.Fatalf("failed to read fixture %s: %v", snap.Fixture, err)
			}

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", snap.ContentType)
				_, _ = w.Write(htmlData)
			}))
			defer server.Close()

			t.Setenv("MCP_FETCH_ALLOW_PRIVATE", "true")

			tool := tools.NewFetchTool()
			ctx := context.Background()
			raw := false
			timeout := 30
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

			for _, expected := range snap.Tests.Contains {
				if !strings.Contains(text, expected) {
					t.Errorf("expected %q in output (first 500 chars: %q)", expected, truncate(text, 500))
				}
			}

			for _, forbidden := range snap.Tests.NotContains {
				if strings.Contains(text, forbidden) {
					t.Errorf("forbidden %q appeared in output", forbidden)
				}
			}

			if snap.Tests.MinLength > 0 && len(text) < snap.Tests.MinLength {
				t.Errorf("output too short: got %d bytes, want at least %d", len(text), snap.Tests.MinLength)
			}

			if len(result.Content) < 2 {
				t.Errorf("expected 2 content blocks (content + metadata), got %d", len(result.Content))
			} else {
				metaContent, ok := result.Content[1].(*mcp.TextContent)
				if !ok {
					t.Errorf("expected *mcp.TextContent for metadata, got %T", result.Content[1])
				} else if !strings.HasPrefix(metaContent.Text, "---\n") {
					t.Errorf("metadata should start with '---\\n', got: %q", truncate(metaContent.Text, 100))
				}
			}

			t.Logf("%s: fixture=%d bytes, output=%d bytes", name, len(htmlData), len(text))
		})
	}
}

func TestIntegration_RedirectNotice(t *testing.T) {
	dest := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("destination content"))
	}))
	defer dest.Close()

	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, dest.URL, http.StatusFound)
	}))
	defer redirector.Close()

	t.Setenv("MCP_FETCH_ALLOW_PRIVATE", "true")

	tool := tools.NewFetchTool()
	ctx := context.Background()
	raw := true
	timeout := 10
	args := tools.FetchArgs{
		URL:     redirector.URL,
		Raw:     &raw,
		Timeout: &timeout,
	}

	result, _, err := tool.Run(ctx, &mcp.CallToolRequest{}, args)
	if err != nil {
		t.Fatalf("fetch failed: %v", err)
	}

	text := textContent(t, result)

	if !strings.Contains(text, "[Note: Redirected to") {
		t.Errorf("expected redirect notice in output, got: %q", truncate(text, 200))
	}
	if !strings.Contains(text, dest.URL) {
		t.Errorf("expected destination URL in redirect notice, got: %q", truncate(text, 200))
	}
	if !strings.Contains(text, "destination content") {
		t.Errorf("expected destination content in output, got: %q", truncate(text, 200))
	}
}

func TestIntegration_MultiHopRedirect(t *testing.T) {
	final := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("final destination"))
	}))
	defer final.Close()

	hop2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, final.URL, http.StatusFound)
	}))
	defer hop2.Close()

	hop1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, hop2.URL, http.StatusFound)
	}))
	defer hop1.Close()

	t.Setenv("MCP_FETCH_ALLOW_PRIVATE", "true")

	tool := tools.NewFetchTool()
	ctx := context.Background()
	raw := true
	timeout := 10
	args := tools.FetchArgs{
		URL:     hop1.URL,
		Raw:     &raw,
		Timeout: &timeout,
	}

	result, _, err := tool.Run(ctx, &mcp.CallToolRequest{}, args)
	if err != nil {
		t.Fatalf("fetch failed: %v", err)
	}

	text := textContent(t, result)

	if !strings.Contains(text, "[Note: Redirected to") {
		t.Errorf("expected redirect notice in output, got: %q", truncate(text, 200))
	}
	if !strings.Contains(text, final.URL) {
		t.Errorf("expected final destination URL in redirect notice, got: %q", truncate(text, 200))
	}
	if !strings.Contains(text, "final destination") {
		t.Errorf("expected final content in output, got: %q", truncate(text, 200))
	}
}

func TestIntegration_TruncationAtParagraphBoundary(t *testing.T) {
	body := `<!DOCTYPE html><html><body><main>
<p>First paragraph with enough text to fill some space here. Lorem ipsum dolor sit amet, consectetur adipiscing elit.</p>

<p>Second paragraph that should be cut off by truncation at paragraph boundary. This text should not appear in output.</p>
</main></body></html>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(body))
	}))
	defer server.Close()

	t.Setenv("MCP_FETCH_ALLOW_PRIVATE", "true")

	tool := tools.NewFetchTool()
	ctx := context.Background()
	maxBytes := 150
	raw := false
	timeout := 10
	args := tools.FetchArgs{
		URL:      server.URL,
		MaxBytes: &maxBytes,
		Raw:      &raw,
		Timeout:  &timeout,
	}

	result, _, err := tool.Run(ctx, &mcp.CallToolRequest{}, args)
	if err != nil {
		t.Fatalf("fetch failed: %v", err)
	}

	text := textContent(t, result)

	if !strings.Contains(text, "First paragraph") {
		t.Errorf("expected first paragraph in output, got: %q", truncate(text, 200))
	}
	if strings.Contains(text, "Second paragraph") {
		t.Errorf("second paragraph should have been truncated, got: %q", truncate(text, 300))
	}
	if !strings.Contains(text, "[Truncated") {
		t.Errorf("expected truncation notice in output, got: %q", truncate(text, 200))
	}
}

func TestIntegration_ContinuationWindow(t *testing.T) {
	content := strings.Repeat("abcdefghij", 100)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte(content))
	}))
	defer server.Close()

	t.Setenv("MCP_FETCH_ALLOW_PRIVATE", "true")

	tool := tools.NewFetchTool()
	ctx := context.Background()
	maxBytes := 50
	startBytes := 200
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

	if !strings.Contains(text, "Continued from byte 200") {
		t.Errorf("expected continuation notice in output, got: %q", truncate(text, 200))
	}
	if !strings.Contains(text, "Truncated") {
		t.Errorf("expected truncation notice in output, got: %q", truncate(text, 200))
	}
}

func TestIntegration_HTTPErrorCode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("Not Found"))
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

	_, _, err := tool.Run(ctx, &mcp.CallToolRequest{}, args)
	if err == nil {
		t.Fatal("expected error for 404 status")
	}
	if !strings.Contains(err.Error(), "http_error") {
		t.Errorf("expected http_error code in error, got: %v", err)
	}
}

func TestIntegration_MetadataFields(t *testing.T) {
	body := "test content for metadata check"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte(body))
	}))
	defer server.Close()

	t.Setenv("MCP_FETCH_ALLOW_PRIVATE", "true")

	tool := tools.NewFetchTool()
	ctx := context.Background()
	raw := true
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

	if len(result.Content) < 2 {
		t.Fatalf("expected 2 content blocks, got %d", len(result.Content))
	}

	metaContent, ok := result.Content[1].(*mcp.TextContent)
	if !ok {
		t.Fatalf("expected *mcp.TextContent for metadata, got %T", result.Content[1])
	}

	metaText := metaContent.Text
	if !strings.Contains(metaText, `"statusCode"`) {
		t.Errorf("metadata should contain statusCode, got: %q", metaText)
	}
	if !strings.Contains(metaText, `"contentBytes"`) {
		t.Errorf("metadata should contain contentBytes, got: %q", metaText)
	}
	if !strings.Contains(metaText, `"fetchedAt"`) {
		t.Errorf("metadata should contain fetchedAt, got: %q", metaText)
	}
	if !strings.Contains(metaText, `"url"`) {
		t.Errorf("metadata should contain url, got: %q", metaText)
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
