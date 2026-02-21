package fetch_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/arawak/mcp-fetch-go/internal/fetch"
)

// newPrivateServer starts an httptest server and sets MCP_FETCH_ALLOW_PRIVATE=true
// so tests can reach 127.0.0.1 without SSRF rejection.
func newPrivateServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	t.Setenv("MCP_FETCH_ALLOW_PRIVATE", "true")
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv
}

// ---- basic fetch ----

func TestFetchURL_PlainText(t *testing.T) {
	srv := newPrivateServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("hello world"))
	})

	got, err := fetch.FetchURL(context.Background(), srv.URL, 1024, 0, true, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Content != "hello world" {
		t.Errorf("expected %q, got %q", "hello world", got.Content)
	}
	if got.Truncated {
		t.Error("expected Truncated=false")
	}
	if got.StatusCode != 200 {
		t.Errorf("expected StatusCode=200, got %d", got.StatusCode)
	}
}

func TestFetchURL_HTMLConverted(t *testing.T) {
	srv := newPrivateServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<html><body><h1>Go</h1><p>Hello.</p></body></html>`))
	})

	got, err := fetch.FetchURL(context.Background(), srv.URL, 4096, 0, false, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(got.Content, "<h1>") {
		t.Errorf("raw HTML should have been converted, got: %q", got.Content)
	}
	if !strings.Contains(got.Content, "Go") || !strings.Contains(got.Content, "Hello") {
		t.Errorf("content should be preserved after conversion, got: %q", got.Content)
	}
}

func TestFetchURL_RawSkipsConversion(t *testing.T) {
	srv := newPrivateServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<html><body><h1>Raw</h1></body></html>`))
	})

	got, err := fetch.FetchURL(context.Background(), srv.URL, 4096, 0, true, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got.Content, "<h1>") {
		t.Errorf("raw mode should preserve HTML tags, got: %q", got.Content)
	}
}

// ---- byte windowing ----

func TestFetchURL_MaxBytes(t *testing.T) {
	// Body has no paragraph/word boundaries so snap falls back to hard cut.
	srv := newPrivateServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("0123456789"))
	})

	got, err := fetch.FetchURL(context.Background(), srv.URL, 5, 0, true, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// ReturnedBytes must be ≤ maxBytes.
	if got.ReturnedBytes > 5 {
		t.Errorf("expected ReturnedBytes≤5, got %d", got.ReturnedBytes)
	}
	if !strings.HasPrefix(got.Content, "01234") {
		t.Errorf("expected content starting with %q, got %q", "01234", got.Content)
	}
}

func TestFetchURL_StartBytes(t *testing.T) {
	srv := newPrivateServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("0123456789"))
	})

	got, err := fetch.FetchURL(context.Background(), srv.URL, 1024, 5, true, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Content begins with the continuation header then the sliced body.
	if !strings.Contains(got.Content, "56789") {
		t.Errorf("expected sliced content %q in output, got %q", "56789", got.Content)
	}
	if !strings.Contains(got.Content, "Continued from byte 5") {
		t.Errorf("expected continuation notice, got %q", got.Content)
	}
}

func TestFetchURL_StartAndMaxBytes(t *testing.T) {
	srv := newPrivateServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("0123456789"))
	})

	got, err := fetch.FetchURL(context.Background(), srv.URL, 3, 4, true, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Content contains the body slice "456" and a continuation notice.
	if !strings.Contains(got.Content, "456") {
		t.Errorf("expected sliced content %q in output, got %q", "456", got.Content)
	}
	if !strings.Contains(got.Content, "Continued from byte 4") {
		t.Errorf("expected continuation notice, got %q", got.Content)
	}
}

func TestFetchURL_StartBeyondEnd(t *testing.T) {
	srv := newPrivateServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("short"))
	})

	got, err := fetch.FetchURL(context.Background(), srv.URL, 1024, 100, true, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Content != "" {
		t.Errorf("expected empty string when startBytes exceeds content, got %q", got.Content)
	}
}

func TestFetchURL_MaxBytesZero(t *testing.T) {
	srv := newPrivateServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("some content"))
	})

	got, err := fetch.FetchURL(context.Background(), srv.URL, 0, 0, true, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Content != "" {
		t.Errorf("expected empty string when maxBytes=0, got %q", got.Content)
	}
}

// ---- truncation metadata ----

func TestFetchURL_TruncationMetadata(t *testing.T) {
	body := strings.Repeat("a", 10000)
	srv := newPrivateServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte(body))
	})

	got, err := fetch.FetchURL(context.Background(), srv.URL, 500, 0, true, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got.Truncated {
		t.Error("expected Truncated=true")
	}
	if got.ContentBytes != 10000 {
		t.Errorf("expected ContentBytes=10000, got %d", got.ContentBytes)
	}
	if got.ReturnedBytes > 500 {
		t.Errorf("expected ReturnedBytes≤500, got %d", got.ReturnedBytes)
	}
	if got.ReturnedBytes == 0 {
		t.Error("expected non-zero ReturnedBytes")
	}
	if got.NextStartBytes == nil || *got.NextStartBytes != got.ReturnedBytes {
		t.Errorf("expected NextStartBytes=%d, got %v", got.ReturnedBytes, got.NextStartBytes)
	}
	if !strings.Contains(got.Content, "to continue") {
		t.Errorf("expected truncation notice in content, got: %q", got.Content)
	}
	if !strings.Contains(got.Content, "of 10000 bytes") {
		t.Errorf("expected byte counts in truncation notice, got: %q", got.Content)
	}
}

func TestFetchURL_NotTruncated(t *testing.T) {
	srv := newPrivateServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("small"))
	})

	got, err := fetch.FetchURL(context.Background(), srv.URL, 1024, 0, true, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Truncated {
		t.Error("expected Truncated=false for small content")
	}
	if got.NextStartBytes != nil {
		t.Errorf("expected NextStartBytes=nil when not truncated, got %v", got.NextStartBytes)
	}
	if strings.Contains(got.Content, "Truncated") {
		t.Errorf("truncation notice should not appear when not truncated, got: %q", got.Content)
	}
}

func TestFetchURL_TruncationWithStartBytes(t *testing.T) {
	body := strings.Repeat("b", 10000)
	srv := newPrivateServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte(body))
	})

	// Start at 5000, read up to 500 → next should be 5000+returnedBytes.
	got, err := fetch.FetchURL(context.Background(), srv.URL, 500, 5000, true, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got.Truncated {
		t.Error("expected Truncated=true")
	}
	expected := 5000 + got.ReturnedBytes
	if got.NextStartBytes == nil || *got.NextStartBytes != expected {
		t.Errorf("expected NextStartBytes=%d, got %v", expected, got.NextStartBytes)
	}
}

// ---- SSRF / validation ----

func TestFetchURL_BlocksPrivateIP(t *testing.T) {
	t.Setenv("MCP_FETCH_ALLOW_PRIVATE", "false")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("blocked"))
	}))
	defer srv.Close()

	_, err := fetch.FetchURL(context.Background(), srv.URL, 1024, 0, true, 10)
	if err == nil {
		t.Fatal("expected error for private IP")
	}
	if !strings.Contains(err.Error(), "private or non-global address") {
		t.Errorf("unexpected error message: %v", err)
	}
	if fetch.ErrorCode(err) != fetch.ErrCodeBlockedDestination {
		t.Errorf("expected error code %q, got %q", fetch.ErrCodeBlockedDestination, fetch.ErrorCode(err))
	}
}

func TestFetchURL_BlocksFileScheme(t *testing.T) {
	_, err := fetch.FetchURL(context.Background(), "file:///etc/passwd", 1024, 0, true, 10)
	if err == nil {
		t.Fatal("expected error for file:// scheme")
	}
	if !strings.Contains(err.Error(), "unsupported scheme") {
		t.Errorf("unexpected error message: %v", err)
	}
	if fetch.ErrorCode(err) != fetch.ErrCodeInvalidURL {
		t.Errorf("expected error code %q, got %q", fetch.ErrCodeInvalidURL, fetch.ErrorCode(err))
	}
}

func TestFetchURL_BlocksDomainNotInAllowlist(t *testing.T) {
	t.Setenv("MCP_FETCH_ALLOWED_DOMAINS", "example.com")
	t.Setenv("MCP_FETCH_ALLOW_PRIVATE", "true")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("blocked"))
	}))
	defer srv.Close()

	_, err := fetch.FetchURL(context.Background(), srv.URL, 1024, 0, true, 10)
	if err == nil {
		t.Fatal("expected error for domain not in allowlist")
	}
	if !strings.Contains(err.Error(), "domain not allowed") {
		t.Errorf("unexpected error message: %v", err)
	}
	if fetch.ErrorCode(err) != fetch.ErrCodeBlockedDestination {
		t.Errorf("expected error code %q, got %q", fetch.ErrCodeBlockedDestination, fetch.ErrorCode(err))
	}
}

func TestFetchURL_Non200Status(t *testing.T) {
	srv := newPrivateServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	_, err := fetch.FetchURL(context.Background(), srv.URL, 1024, 0, true, 10)
	if err == nil {
		t.Fatal("expected error for 404 response")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("unexpected error message: %v", err)
	}
	if fetch.ErrorCode(err) != fetch.ErrCodeHTTPError {
		t.Errorf("expected error code %q, got %q", fetch.ErrCodeHTTPError, fetch.ErrorCode(err))
	}
}

func TestFetchURL_Non200Status500(t *testing.T) {
	srv := newPrivateServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	_, err := fetch.FetchURL(context.Background(), srv.URL, 1024, 0, true, 10)
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}

func TestFetchURL_InvalidURL(t *testing.T) {
	_, err := fetch.FetchURL(context.Background(), "://bad-url", 1024, 0, true, 10)
	if err == nil {
		t.Fatal("expected error for invalid URL")
	}
}

func TestFetchURL_InvalidByteRange(t *testing.T) {
	_, err := fetch.FetchURL(context.Background(), "https://example.com", -1, 0, true, 10)
	if err == nil {
		t.Fatal("expected error for negative maxBytes")
	}
}

// ---- content type gating ----

func TestFetchURL_UnsupportedContentType(t *testing.T) {
	srv := newPrivateServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("\x89PNG"))
	})

	_, err := fetch.FetchURL(context.Background(), srv.URL, 1024, 0, false, 10)
	if err == nil {
		t.Fatal("expected error for unsupported content type")
	}
	if !strings.Contains(err.Error(), "unsupported content type") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestFetchURL_UnsupportedContentType_PDFBlocked(t *testing.T) {
	srv := newPrivateServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/pdf")
		_, _ = w.Write([]byte("%PDF"))
	})

	_, err := fetch.FetchURL(context.Background(), srv.URL, 1024, 0, false, 10)
	if err == nil {
		t.Fatal("expected error for application/pdf")
	}
}

func TestFetchURL_RawModeAllowsAnyContentType(t *testing.T) {
	// raw=true skips content-type check so callers can get raw bytes of anything.
	srv := newPrivateServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write([]byte("raw bytes"))
	})

	got, err := fetch.FetchURL(context.Background(), srv.URL, 1024, 0, true, 10)
	if err != nil {
		t.Fatalf("unexpected error in raw mode for unknown content type: %v", err)
	}
	if got.Content != "raw bytes" {
		t.Errorf("expected raw bytes, got %q", got.Content)
	}
}

// ---- structured error codes ----

func TestFetchURL_ErrorCode_InvalidURL(t *testing.T) {
	_, err := fetch.FetchURL(context.Background(), "://bad", 1024, 0, true, 10)
	if err == nil {
		t.Fatal("expected error")
	}
	if fetch.ErrorCode(err) != fetch.ErrCodeInvalidURL {
		t.Errorf("expected %q, got %q", fetch.ErrCodeInvalidURL, fetch.ErrorCode(err))
	}
}

func TestFetchURL_ErrorCode_BlockedDestination(t *testing.T) {
	t.Setenv("MCP_FETCH_ALLOW_PRIVATE", "false")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()

	_, err := fetch.FetchURL(context.Background(), srv.URL, 1024, 0, true, 10)
	if err == nil {
		t.Fatal("expected error")
	}
	if fetch.ErrorCode(err) != fetch.ErrCodeBlockedDestination {
		t.Errorf("expected %q, got %q", fetch.ErrCodeBlockedDestination, fetch.ErrorCode(err))
	}
}

func TestFetchURL_ErrorCode_HTTPError(t *testing.T) {
	srv := newPrivateServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	})

	_, err := fetch.FetchURL(context.Background(), srv.URL, 1024, 0, true, 10)
	if err == nil {
		t.Fatal("expected error")
	}
	if fetch.ErrorCode(err) != fetch.ErrCodeHTTPError {
		t.Errorf("expected %q, got %q", fetch.ErrCodeHTTPError, fetch.ErrorCode(err))
	}
	// Details should include status code.
	fe, ok := err.(*fetch.FetchError)
	if !ok {
		t.Fatalf("expected *fetch.FetchError, got %T", err)
	}
	if fe.Details["status_code"] != 403 {
		t.Errorf("expected status_code=403 in details, got %v", fe.Details["status_code"])
	}
}

func TestFetchURL_ErrorCode_UnsupportedContentType(t *testing.T) {
	srv := newPrivateServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("\x89PNG"))
	})

	_, err := fetch.FetchURL(context.Background(), srv.URL, 1024, 0, false, 10)
	if err == nil {
		t.Fatal("expected error")
	}
	if fetch.ErrorCode(err) != fetch.ErrCodeUnsupportedType {
		t.Errorf("expected %q, got %q", fetch.ErrCodeUnsupportedType, fetch.ErrorCode(err))
	}
}

func TestFetchURL_ErrorCode_InvalidRange(t *testing.T) {
	_, err := fetch.FetchURL(context.Background(), "https://example.com", -1, 0, true, 10)
	if err == nil {
		t.Fatal("expected error")
	}
	if fetch.ErrorCode(err) != fetch.ErrCodeInvalidRange {
		t.Errorf("expected %q, got %q", fetch.ErrCodeInvalidRange, fetch.ErrorCode(err))
	}
}

// ---- request headers ----

func TestFetchURL_SetsUserAgent(t *testing.T) {
	var gotUA string
	srv := newPrivateServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("ok"))
	})

	_, err := fetch.FetchURL(context.Background(), srv.URL, 1024, 0, true, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(gotUA, "mcp-fetch-go/") {
		t.Errorf("expected mcp-fetch-go User-Agent, got %q", gotUA)
	}
}

func TestFetchURL_SetsAcceptHeader(t *testing.T) {
	var gotAccept string
	srv := newPrivateServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotAccept = r.Header.Get("Accept")
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("ok"))
	})

	_, err := fetch.FetchURL(context.Background(), srv.URL, 1024, 0, true, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(gotAccept, "text/html") {
		t.Errorf("expected Accept to include text/html, got %q", gotAccept)
	}
	if !strings.Contains(gotAccept, "text/plain") {
		t.Errorf("expected Accept to include text/plain, got %q", gotAccept)
	}
}

// ---- FinalURL and StatusCode ----

func TestFetchURL_FinalURL_AfterRedirect(t *testing.T) {
	t.Setenv("MCP_FETCH_ALLOW_PRIVATE", "true")

	dest := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("destination"))
	}))
	defer dest.Close()

	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, dest.URL, http.StatusFound)
	}))
	defer redirector.Close()

	got, err := fetch.FetchURL(context.Background(), redirector.URL, 1024, 0, true, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.FinalURL != dest.URL+"/" && got.FinalURL != dest.URL {
		t.Errorf("expected FinalURL=%q, got %q", dest.URL, got.FinalURL)
	}
}

func TestFetchURL_StatusCode(t *testing.T) {
	srv := newPrivateServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	got, err := fetch.FetchURL(context.Background(), srv.URL, 1024, 0, true, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.StatusCode != 200 {
		t.Errorf("expected StatusCode=200, got %d", got.StatusCode)
	}
}

func TestFetchURL_FetchedAt(t *testing.T) {
	srv := newPrivateServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("ok"))
	})

	got, err := fetch.FetchURL(context.Background(), srv.URL, 1024, 0, true, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.FetchedAt == "" {
		t.Error("expected FetchedAt to be set")
	}
	// Should be RFC3339 — contains 'T' and 'Z' or offset.
	if !strings.Contains(got.FetchedAt, "T") {
		t.Errorf("FetchedAt doesn't look like RFC3339: %q", got.FetchedAt)
	}
}

// ---- paragraph-boundary snapping ----

func TestFetchURL_SnapsToParagraphBoundary(t *testing.T) {
	// Body has a paragraph break well before maxBytes.
	// "para one\n\npara two long enough to push past maxBytes"
	body := "para one\n\n" + strings.Repeat("x", 500)
	srv := newPrivateServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte(body))
	})

	got, err := fetch.FetchURL(context.Background(), srv.URL, 200, 0, true, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should snap back to end of "para one\n\n" boundary.
	if !strings.Contains(got.Content, "para one") {
		t.Errorf("expected first paragraph in output, got: %q", got.Content)
	}
	// Should not contain any of the second paragraph's x's.
	if strings.Contains(got.Content[:got.ReturnedBytes], "x") {
		t.Errorf("should have snapped before second paragraph, got: %q", got.Content[:got.ReturnedBytes])
	}
	if got.ReturnedBytes >= 200 {
		// snapped to boundary, must be less than maxBytes
		t.Errorf("expected ReturnedBytes < 200, got %d", got.ReturnedBytes)
	}
}

func TestFetchURL_SnapsToWordBoundary(t *testing.T) {
	// No paragraph breaks but has spaces.
	body := strings.Repeat("word ", 100) // 500 bytes, spaces every 5
	srv := newPrivateServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte(body))
	})

	got, err := fetch.FetchURL(context.Background(), srv.URL, 22, 0, true, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Returned content must end at a word boundary (space), not mid-word.
	content := got.Content[:got.ReturnedBytes]
	if len(content) > 0 && content[len(content)-1] != ' ' {
		// Allow trailing space or end-of-word; just must not end mid-"word".
		if strings.HasSuffix(content, "wor") || strings.HasSuffix(content, "wo") || strings.HasSuffix(content, "w") {
			t.Errorf("truncated mid-word: %q", content)
		}
	}
}

func TestFetchURL_ContinuationNotice(t *testing.T) {
	body := strings.Repeat("b", 10000)
	srv := newPrivateServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte(body))
	})

	got, err := fetch.FetchURL(context.Background(), srv.URL, 1000, 5000, true, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got.Content, "Continued from byte 5000 of 10000") {
		t.Errorf("expected continuation notice, got: %q", got.Content[:300])
	}
}

func TestFetchURL_NoContinuationNoticeOnFirstChunk(t *testing.T) {
	srv := newPrivateServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("hello"))
	})

	got, err := fetch.FetchURL(context.Background(), srv.URL, 1024, 0, true, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(got.Content, "Continued from") {
		t.Errorf("should not have continuation notice on first chunk, got: %q", got.Content)
	}
}

// ---- effective_go real-world integration ----

func TestFetchURL_EffectiveGoHTML(t *testing.T) {
	data, err := os.ReadFile("../../tests/testdata/effective_go.html")
	if err != nil {
		t.Fatalf("failed to read testdata: %v", err)
	}

	srv := newPrivateServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(data)
	})

	got, err := fetch.FetchURL(context.Background(), srv.URL, 200000, 0, false, 30)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	md := got.Content

	// Key section headings must survive conversion.
	for _, heading := range []string{"Introduction", "Formatting", "Commentary", "Names", "Semicolons"} {
		if !strings.Contains(md, heading) {
			t.Errorf("expected heading %q in Markdown output", heading)
		}
	}

	// Nav noise must be gone.
	for _, noise := range []string{"Why Go", "Packages", "<nav", "<footer", "<script"} {
		if strings.Contains(md, noise) {
			t.Errorf("nav/noise text %q should not appear in output", noise)
		}
	}

	// No raw HTML tags.
	if strings.Contains(md, "<h2") || strings.Contains(md, "<p>") {
		t.Errorf("raw HTML tags should not appear in Markdown output")
	}

	// Output should be substantial.
	if got.ContentBytes < 10000 {
		t.Errorf("ContentBytes too small (%d); content may be lost", got.ContentBytes)
	}
	if got.Truncated {
		t.Error("should not be truncated with maxBytes=200000")
	}

	t.Logf("effective_go: HTML=%d bytes -> Markdown=%d bytes", len(data), got.ContentBytes)
}

func TestFetchURL_EffectiveGoByteWindow(t *testing.T) {
	data, err := os.ReadFile("../../tests/testdata/effective_go.html")
	if err != nil {
		t.Fatalf("failed to read testdata: %v", err)
	}

	srv := newPrivateServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(data)
	})

	// First window.
	first, err := fetch.FetchURL(context.Background(), srv.URL, 500, 0, false, 30)
	if err != nil {
		t.Fatalf("unexpected error (first window): %v", err)
	}
	if !first.Truncated {
		t.Error("expected Truncated=true for first window")
	}
	// Paragraph snapping means ReturnedBytes may be ≤ 500.
	if first.ReturnedBytes > 500 || first.ReturnedBytes == 0 {
		t.Errorf("expected 0 < ReturnedBytes ≤ 500, got %d", first.ReturnedBytes)
	}
	if first.NextStartBytes == nil || *first.NextStartBytes != first.ReturnedBytes {
		t.Errorf("expected NextStartBytes=%d, got %v", first.ReturnedBytes, first.NextStartBytes)
	}
	if !strings.Contains(first.Content, "to continue") {
		t.Errorf("expected truncation notice, got: %q", first.Content[:200])
	}

	// Second window using NextStartBytes: should begin with continuation notice.
	second, err := fetch.FetchURL(context.Background(), srv.URL, 500, *first.NextStartBytes, false, 30)
	if err != nil {
		t.Fatalf("unexpected error (second window): %v", err)
	}
	if second.ReturnedBytes > 500 || second.ReturnedBytes == 0 {
		t.Errorf("expected 0 < ReturnedBytes ≤ 500 for second window, got %d", second.ReturnedBytes)
	}
	if !strings.Contains(second.Content, "Continued from byte") {
		t.Errorf("expected continuation notice in second window, got: %q", second.Content[:200])
	}

	// ContentBytes should be the same for both windows (same document).
	if first.ContentBytes != second.ContentBytes {
		t.Errorf("ContentBytes should be consistent: first=%d, second=%d",
			first.ContentBytes, second.ContentBytes)
	}

	t.Logf("effective_go: total=%d bytes, window1=%d, window2=%d",
		first.ContentBytes, first.ReturnedBytes, second.ReturnedBytes)
}
