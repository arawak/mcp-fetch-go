package fetch

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/arawak/mcp-fetch-go/internal/convert"
	"github.com/arawak/mcp-fetch-go/internal/tlsroots"
)

const defaultMaxRedirects = 5

// supportedContentTypes lists MIME prefixes we are willing to process.
var supportedContentTypes = []string{
	"text/html",
	"application/xhtml+xml",
	"text/plain",
	"text/markdown",
}

type fetchOptions struct {
	allowedDomains []string
	allowPrivate   bool
	maxRedirects   int
}

func optionsFromEnv() fetchOptions {
	allowedDomains := parseEnvList("MCP_FETCH_ALLOWED_DOMAINS")
	allowPrivate := parseEnvBool("MCP_FETCH_ALLOW_PRIVATE")
	maxRedirects := defaultMaxRedirects
	if value := strings.TrimSpace(os.Getenv("MCP_FETCH_MAX_REDIRECTS")); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil && parsed >= 0 {
			maxRedirects = parsed
		}
	}
	return fetchOptions{
		allowedDomains: allowedDomains,
		allowPrivate:   allowPrivate,
		maxRedirects:   maxRedirects,
	}
}

func parseEnvList(key string) []string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.ToLower(strings.TrimSpace(part))
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func parseEnvBool(key string) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return false
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false
	}
	return parsed
}

func domainAllowed(host string, allowedDomains []string) bool {
	if len(allowedDomains) == 0 {
		return true
	}
	candidate := strings.ToLower(host)
	for _, domain := range allowedDomains {
		if candidate == domain {
			return true
		}
		if strings.HasSuffix(candidate, "."+domain) {
			return true
		}
	}
	return false
}

func isAllowedIP(ip net.IP) bool {
	if ip == nil {
		return false
	}
	if !ip.IsGlobalUnicast() {
		return false
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified() {
		return false
	}
	return true
}

func validateURL(ctx context.Context, target *url.URL, opts fetchOptions) error {
	if target == nil {
		return newFetchError(ErrCodeInvalidURL, "missing url", nil)
	}
	if target.Scheme != "http" && target.Scheme != "https" {
		return newFetchError(ErrCodeInvalidURL, "unsupported scheme: "+target.Scheme,
			map[string]any{"scheme": target.Scheme})
	}
	host := target.Hostname()
	if host == "" {
		return newFetchError(ErrCodeInvalidURL, "missing host", nil)
	}
	if !domainAllowed(host, opts.allowedDomains) {
		return newFetchError(ErrCodeBlockedDestination, "domain not allowed: "+host,
			map[string]any{"host": host, "reason": "allowlist"})
	}
	if opts.allowPrivate {
		return nil
	}
	if ip := net.ParseIP(host); ip != nil {
		if !isAllowedIP(ip) {
			return newFetchError(ErrCodeBlockedDestination, "private or non-global address: "+host,
				map[string]any{"host": host, "ip": host, "reason": "private_ip"})
		}
		return nil
	}
	addrs, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return newFetchError(ErrCodeNetworkError, "DNS lookup failed: "+err.Error(),
			map[string]any{"host": host})
	}
	if len(addrs) == 0 {
		return newFetchError(ErrCodeNetworkError, "no addresses for host: "+host,
			map[string]any{"host": host})
	}
	for _, addr := range addrs {
		if !isAllowedIP(addr.IP) {
			return newFetchError(ErrCodeBlockedDestination, "private or non-global address: "+addr.IP.String(),
				map[string]any{"host": host, "ip": addr.IP.String(), "reason": "private_ip"})
		}
	}
	return nil
}

func isHTMLContentType(value string) bool {
	ct := strings.ToLower(strings.TrimSpace(value))
	return strings.HasPrefix(ct, "text/html") || strings.HasPrefix(ct, "application/xhtml+xml")
}

// isSupportedContentType returns true if the content-type is one we can handle.
func isSupportedContentType(value string) bool {
	ct := strings.ToLower(strings.TrimSpace(value))
	// Strip parameters (e.g. "; charset=utf-8")
	if i := strings.Index(ct, ";"); i >= 0 {
		ct = strings.TrimSpace(ct[:i])
	}
	for _, supported := range supportedContentTypes {
		if ct == supported {
			return true
		}
	}
	return false
}

// FetchURL fetches targetURL and returns a structured FetchResult.
// Errors are returned as *FetchError with a stable Code field; use ErrorCode()
// to extract the code without a type assertion.
func FetchURL(ctx context.Context, targetURL string, maxBytes int, startBytes int, raw bool, timeoutSeconds int) (*FetchResult, error) {
	if maxBytes < 0 || startBytes < 0 {
		return nil, newFetchError(ErrCodeInvalidRange, "maxBytes and startBytes must be non-negative", nil)
	}

	opts := optionsFromEnv()
	parsedURL, err := url.Parse(targetURL)
	if err != nil {
		return nil, newFetchError(ErrCodeInvalidURL, "invalid URL: "+err.Error(), map[string]any{"url": targetURL})
	}
	if err := validateURL(ctx, parsedURL, opts); err != nil {
		return nil, err
	}

	transport := tlsroots.NewTransport()
	client := &http.Client{
		Timeout:   time.Duration(timeoutSeconds) * time.Second,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if opts.maxRedirects >= 0 && len(via) > opts.maxRedirects {
				return newFetchError(ErrCodeNetworkError,
					fmt.Sprintf("stopped after %d redirects", opts.maxRedirects),
					map[string]any{"max_redirects": opts.maxRedirects})
			}
			return validateURL(req.Context(), req.URL, opts)
		},
	}

	req, err := http.NewRequestWithContext(ctx, "GET", parsedURL.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "mcp-fetch-go/1.0 (+https://github.com/mcp-fetch-go)")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,text/plain,text/markdown;q=0.9,*/*;q=0.1")

	resp, err := client.Do(req)
	if err != nil {
		// Classify context cancellation vs. generic network errors.
		if ctx.Err() != nil {
			return nil, newFetchError(ErrCodeTimeout, "request timed out or cancelled: "+err.Error(), nil)
		}
		return nil, newFetchError(ErrCodeNetworkError, err.Error(), nil)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, newFetchError(ErrCodeHTTPError,
			fmt.Sprintf("unexpected status: %s", resp.Status),
			map[string]any{"status_code": resp.StatusCode, "status": resp.Status})
	}

	contentType := resp.Header.Get("Content-Type")
	if !raw && !isSupportedContentType(contentType) {
		return nil, newFetchError(ErrCodeUnsupportedType,
			"unsupported content type: "+contentType,
			map[string]any{"content_type": contentType})
	}

	// Read the full body up to an internal cap so the HTML parser always sees a
	// complete document. The byte window is applied after conversion.
	const internalCap = 10 * 1024 * 1024 // 10 MiB
	body, err := io.ReadAll(io.LimitReader(resp.Body, internalCap))
	if err != nil {
		return nil, newFetchError(ErrCodeNetworkError, "error reading response body: "+err.Error(), nil)
	}

	content := string(body)
	if !raw && isHTMLContentType(contentType) {
		content, err = convert.ConvertHTMLToMarkdown(content)
		if err != nil {
			return nil, newFetchError(ErrCodeConversionFailed, "HTML to Markdown conversion failed: "+err.Error(), nil)
		}
	}

	contentBytes := len(content)

	// Apply byte window to the final content.
	if maxBytes == 0 {
		return &FetchResult{
			URL:           targetURL,
			FinalURL:      resp.Request.URL.String(),
			StatusCode:    resp.StatusCode,
			ContentType:   contentType,
			Content:       "",
			ContentBytes:  contentBytes,
			ReturnedBytes: 0,
			Truncated:     contentBytes > 0,
			NextStartBytes: func() *int {
				v := 0
				if contentBytes > 0 {
					return &v
				}
				return nil
			}(),
			FetchedAt: time.Now().UTC().Format(time.RFC3339),
		}, nil
	}

	finalURL := resp.Request.URL.String()
	fetchedAt := time.Now().UTC().Format(time.RFC3339)

	if startBytes > 0 {
		if startBytes >= contentBytes {
			return &FetchResult{
				URL:           targetURL,
				FinalURL:      finalURL,
				StatusCode:    resp.StatusCode,
				ContentType:   contentType,
				Content:       "",
				ContentBytes:  contentBytes,
				ReturnedBytes: 0,
				Truncated:     false,
				FetchedAt:     fetchedAt,
			}, nil
		}
		content = content[startBytes:]
	}

	// Snap truncation to a paragraph boundary so callers never receive a
	// mid-word or mid-sentence fragment.
	truncated := len(content) > maxBytes
	if truncated {
		content = snapToParagraph(content, maxBytes)
	}

	returnedBytes := len(content)

	// Prepend a continuation notice when this is not the first chunk.
	if startBytes > 0 {
		content = fmt.Sprintf("[Continued from byte %d of %d]\n\n", startBytes, contentBytes) + content
	}

	// Append truncation notice so callers know how to page forward.
	if truncated {
		nextOff := startBytes + returnedBytes
		content += fmt.Sprintf("\n\n---\n[Truncated: %d of %d bytes. Use startBytes=%d to continue.]",
			returnedBytes, contentBytes, nextOff)
	}

	var nextStartBytes *int
	if truncated {
		v := startBytes + returnedBytes
		nextStartBytes = &v
	}

	return &FetchResult{
		URL:            targetURL,
		FinalURL:       finalURL,
		StatusCode:     resp.StatusCode,
		ContentType:    contentType,
		Content:        content,
		ContentBytes:   contentBytes,
		ReturnedBytes:  returnedBytes,
		Truncated:      truncated,
		NextStartBytes: nextStartBytes,
		FetchedAt:      fetchedAt,
	}, nil
}

// snapToParagraph returns content[:maxBytes] snapped back to the last paragraph
// boundary (double newline) that falls within the limit. If no paragraph
// boundary exists within the limit it falls back to the last word boundary
// (single space), and if that also cannot be found it returns exactly
// content[:maxBytes].
func snapToParagraph(content string, maxBytes int) string {
	if maxBytes >= len(content) {
		return content
	}
	window := content[:maxBytes]

	// Prefer paragraph boundary.
	if idx := strings.LastIndex(window, "\n\n"); idx > 0 {
		return content[:idx]
	}
	// Fall back to word boundary.
	if idx := strings.LastIndexByte(window, ' '); idx > 0 {
		return content[:idx]
	}
	// Hard cut.
	return window
}
