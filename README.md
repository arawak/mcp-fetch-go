# gofetch

A Model Context Protocol (MCP) server that fetches URLs and converts HTML to clean Markdown. Designed specifically for LLM-based coding assistants.

## Why gofetch?

Unlike generic web scrapers, gofetch understands that LLMs work best with:

- **Clean Markdown** — Not raw HTML or word-wrapped text
- **Structured metadata** — Truncation status, final URL after redirects, content type
- **Smart truncation** — Paragraph boundaries, not mid-sentence cuts
- **Clear pagination** — Next byte positions, continuation notices

### Goals

1. **Extract content, not noise** — Strip navigation, sidebars, footers, and UI elements automatically
2. **Provide structured output** — Return Markdown + JSON metadata for programmatic use
3. **Handle real-world scenarios** — Detect redirects, block attempts, unsupported content types
4. **Optimize for LLM context** — Truncate intelligently at paragraph boundaries to preserve readability
5. **Enable pagination** — Support byte-range fetching with clear indicators for continuing multi-page documents

## Comparison: gofetch vs. pyfetch

| Aspect | gofetch | pyfetch |
|--------|---------|---------|
| Markdown quality | Excellent | Plain text only |
| Noise stripping | Best-in-class | Basic |
| Redirect detection | Shows final URL | Silent |
| Metadata | Full JSON | None |
| Truncation | Paragraph-aware | Arbitrary byte boundary |
| Pagination notices | Clear (`[Continued from byte X]`) | None |
| Site compatibility | High (works on most dev sites) | Low (blocked by many) |
| robots.txt | Ignores | Respects |

**Verdict:** gofetch is purpose-built for LLM coding workflows; pyfetch is an ethical general-purpose scraper.

## Installation

### From Source

```bash
git clone https://github.com/arawak/mcp-fetch-go.git
cd mcp-fetch-go
go build -o gofetch ./cmd/mcp-fetch-go
```

### With Go Install

```bash
go install github.com/arawak/mcp-fetch-go/cmd/mcp-fetch-go@latest
```

## Usage

### MCP Configuration

Add to your MCP client configuration (Claude Desktop, Cursor, Windsurf, etc.):

```json
{
  "mcpServers": {
    "fetch": {
      "command": "gofetch"
    }
  }
}
```

### Tool: `fetch`

Fetches a URL and returns its content as Markdown (or raw text).

**Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `url` | string | *required* | URL to fetch |
| `maxBytes` | int | 30000 | Maximum bytes to return |
| `startBytes` | int | 0 | Bytes to skip (for pagination) |
| `raw` | bool | false | Return raw content without Markdown conversion |
| `timeout` | int | 30 | Timeout in seconds |

**Example:**

```json
{
  "url": "https://go.dev/doc/effective_go",
  "maxBytes": 5000
}
```

**Response:**

The tool returns two content blocks:

1. **Markdown content** — The converted content (or raw if `raw: true`)
2. **Metadata JSON** — Information about the fetch:

```json
{
  "url": "https://go.dev/doc/effective_go",
  "statusCode": 200,
  "contentType": "text/html",
  "contentBytes": 45123,
  "returnedBytes": 5000,
  "truncated": true,
  "nextStartBytes": 5000,
  "fetchedAt": "2026-02-21T10:30:00Z"
}
```

### Redirect Handling

When a URL redirects, gofetch prepends a notice to the content:

```
[Note: Redirected to https://final-url.example.com/path]

...content...
```

This allows LLMs to know the actual source of the content.

### Pagination

For large documents, use `startBytes` to continue reading:

```json
// First request
{"url": "https://example.com/large-doc", "maxBytes": 10000}

// Response includes: "nextStartBytes": 10000
// Continue from where you left off:
{"url": "https://example.com/large-doc", "maxBytes": 10000, "startBytes": 10000}
```

Truncated content includes a notice:

```
...content...

---
[Truncated: 10000 of 50000 bytes. Use startBytes=10000 to continue.]
```

## Content Processing

gofetch extracts clean content from HTML through a multi-stage pipeline:

1. **Find main content** — Looks for `<main>` or `<article>` elements first; falls back to full document
2. **Strip structural noise** — Removes `<nav>`, `<footer>`, `<aside>`, `<header>`, `<form>`, `<script>`, `<style>`, `<iframe>`, `<svg>`
3. **Strip ARIA noise** — Removes elements with `role="navigation"`, `role="banner"`, `role="contentinfo"`, `role="button"`, etc.
4. **Strip by class** — Removes elements with classes containing: `sidebar`, `breadcrumb`, `cookie`, `advertisement`, `navbar`, `toc`
5. **Strip UI chrome** — Removes buttons, images, and elements with `aria-hidden="true"`
6. **Clean web components** — Unwraps unknown custom elements, removes known noise components (GitHub's `tool-tip`, `action-menu`, `copy-button`; Reddit's `faceplate-*`, `shreddit-*`)
7. **Convert to Markdown** — Uses [html-to-markdown](https://github.com/JohannesKaufmann/html-to-markdown) with CommonMark output

## Supported Content Types

- `text/html` — Converted to Markdown
- `application/xhtml+xml` — Converted to Markdown
- `text/plain` — Returned as-is
- `text/markdown` — Returned as-is

Other content types return an error. Use `raw: true` to bypass content-type checks and return raw bytes.

## Security Configuration

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `MCP_FETCH_ALLOWED_DOMAINS` | *(empty)* | Comma-separated allowlist of domains. Empty = all allowed |
| `MCP_FETCH_ALLOW_PRIVATE` | `false` | Allow private/loopback IPs (SSRF protection) |
| `MCP_FETCH_MAX_REDIRECTS` | `5` | Maximum redirects to follow |

### SSRF Protection

By default, the server blocks requests to:

- Private IP addresses (`10.0.0.0/8`, `172.16.0.0/12`, `192.168.0.0/16`)
- Loopback addresses (`127.0.0.0/8`, `::1`)
- Link-local addresses (`169.254.0.0/16`, `fe80::/10`)

To allow private IPs (for development or internal networks):

```bash
MCP_FETCH_ALLOW_PRIVATE=true gofetch
```

To restrict to specific domains:

```bash
MCP_FETCH_ALLOWED_DOMAINS=example.com,docs.example.com gofetch
```

## Building

```bash
# Build binary
go build -o gofetch ./cmd/mcp-fetch-go

# Build for Docker (static binary)
CGO_ENABLED=0 go build -ldflags="-s -w" -o gofetch ./cmd/mcp-fetch-go
```

### Docker

```dockerfile
FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o gofetch ./cmd/mcp-fetch-go

FROM scratch
COPY --from=builder /app/gofetch /gofetch
ENTRYPOINT ["/gofetch"]
```

## Testing

```bash
# Run all tests
go test ./...

# Run with coverage
go test -cover ./...

# Run integration tests only
go test ./tests/... -v
```

## License

MIT License — See [LICENSE](LICENSE) for details.