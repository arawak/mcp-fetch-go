# mcp-fetch-go

A Model Context Protocol (MCP) server that fetches URLs and converts HTML to clean Markdown. Designed for LLMs and AI assistants that need to read web content.

## Features

- **HTML to Markdown conversion** — Automatically converts HTML pages to clean, readable Markdown
- **Content extraction** — Extracts main content from `<main>` or `<article>` elements, strips navigation, sidebars, footers, and UI noise
- **Byte windowing** — Fetch partial content with `maxBytes` and `startBytes` for large documents
- **Paragraph-aware truncation** — Truncates at paragraph boundaries when possible
- **Redirect tracking** — Reports final URL after redirects
- **SSRF protection** — Blocks private IPs by default, configurable allowlist
- **Container-friendly** — Includes Mozilla CA bundle for TLS in distroless/scratch containers

## Installation

### From Source

```bash
git clone https://github.com/anomalyco/mcp-fetch-go.git
cd mcp-fetch-go
go build -o mcp-fetch-go ./cmd/mcp-fetch-go
```

### With Go Install

```bash
go install mcp-fetch-go@latest
```

## Usage

### MCP Configuration

Add to your MCP client configuration (e.g., Claude Desktop, Cursor, etc.):

```json
{
  "mcpServers": {
    "fetch": {
      "command": "mcp-fetch-go",
      "args": []
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

### Pagination Example

For large documents, use `startBytes` to continue reading:

```json
// First request
{"url": "https://example.com/large-doc", "maxBytes": 10000}

// Continue from where you left off
{"url": "https://example.com/large-doc", "maxBytes": 10000, "startBytes": 10000}
```

## Security Configuration

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `MCP_FETCH_ALLOWED_DOMAINS` | *(empty)* | Comma-separated allowlist of domains. Empty = all allowed |
| `MCP_FETCH_ALLOW_PRIVATE` | `false` | Allow private/loopback IPs (SSRF protection) |
| `MCP_FETCH_MAX_REDIRECTS` | `5` | Maximum redirects to follow |

### SSRF Protection

By default, the server blocks requests to:
- Private IP addresses (10.x, 172.16-31.x, 192.168.x)
- Loopback addresses (127.x, ::1)
- Link-local addresses (169.254.x, fe80::)

To allow private IPs (for development or internal networks):

```bash
MCP_FETCH_ALLOW_PRIVATE=true mcp-fetch-go
```

To restrict to specific domains:

```bash
MCP_FETCH_ALLOWED_DOMAINS=example.com,docs.example.com mcp-fetch-go
```

## Building

```bash
# Build binary
go build -o mcp-fetch-go ./cmd/mcp-fetch-go

# Build for Docker (static binary)
CGO_ENABLED=0 go build -ldflags="-s -w" -o mcp-fetch-go ./cmd/mcp-fetch-go
```

### Docker

```dockerfile
FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o mcp-fetch-go ./cmd/mcp-fetch-go

FROM scratch
COPY --from=builder /app/mcp-fetch-go /mcp-fetch-go
ENTRYPOINT ["/mcp-fetch-go"]
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

## Content Processing

The server extracts clean content from HTML by:

1. **Finding main content** — Looks for `<main>` or `<article>` elements first
2. **Stripping noise** — Removes `<nav>`, `<footer>`, `<aside>`, `<script>`, `<style>`, and elements with navigation-related ARIA roles
3. **Cleaning custom elements** — Unwraps unknown custom elements, removes known UI noise components (GitHub's `tool-tip`, Reddit's `faceplate-*`, etc.)
4. **Converting to Markdown** — Uses [html-to-markdown](https://github.com/JohannesKaufmann/html-to-markdown) with CommonMark output

## Supported Content Types

- `text/html` — Converted to Markdown
- `application/xhtml+xml` — Converted to Markdown
- `text/plain` — Returned as-is
- `text/markdown` — Returned as-is

Other content types return an error (use `raw: true` to bypass).

## License

MIT License — See [LICENSE](LICENSE) for details.