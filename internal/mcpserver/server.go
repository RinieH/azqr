// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package mcpserver

import (
	"fmt"

	"github.com/mark3labs/mcp-go/server"
)

var s *server.MCPServer

// mcp starts the MCP server using mark3labs/mcp-go.
// It registers the azqr tools and serves requests over stdio.
func Start() {
	s = server.NewMCPServer(
		"Azure Quick Review 🚀",
		"1.0.0",
		server.WithToolCapabilities(false),
		server.WithToolCapabilities(true), // Enable tool notifications
		server.WithResourceCapabilities(true, true),
		server.WithPromptCapabilities(true),
		server.WithRecovery(), // Graceful panic recovery
	)

	RegisterAnalysisPrompts(s)
	RegsiterTools(s)

	// Start the stdio server
	if err := server.ServeStdio(s); err != nil {
		fmt.Printf("Server error: %v\n", err)
	}
}
