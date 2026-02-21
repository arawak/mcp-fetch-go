package main

import (
	"context"
	"log"

	"github.com/arawak/mcp-fetch-go/internal/tools"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	// Embed the Mozilla CA bundle so the binary works in minimal container
	// environments (scratch, distroless, Alpine) that lack a system cert store.
	_ "golang.org/x/crypto/x509roots/fallback"
)

func main() {
	ctx := context.Background()

	server := mcp.NewServer(&mcp.Implementation{Name: "mcp-fetch-go", Version: "v0.1.0"}, nil)
	mcp.AddTool(server, &mcp.Tool{Name: "fetch", Description: "Fetch a URL and return content"}, tools.NewFetchTool().Run)

	if err := server.Run(ctx, &mcp.StdioTransport{}); err != nil {
		log.Fatalf("Failed to run server: %v", err)
	}
}
