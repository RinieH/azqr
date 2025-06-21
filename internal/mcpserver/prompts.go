// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package mcpserver

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func RegisterAnalysisPrompts(s *server.MCPServer) {
	scanPrompt := mcp.NewPrompt(
		"scan_subscription",
		mcp.WithPromptDescription("Comprehensive azqr scan for an Azure subscription"),
		mcp.WithArgument("subscription_id", mcp.RequiredArgument()),
	)

	s.AddPrompt(scanPrompt, handleScanPrompt())
}

func handleScanPrompt() func(context.Context, mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	return func(ctx context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		subID := request.Params.Arguments["subscription_id"]
		prompt := `Perform a comprehensive azqr scan for Azure subscription %s.

Please:
1. Scan the subscription using the scan tool
`

		promptText := fmt.Sprintf(prompt, subID)
		promptMessage := mcp.NewPromptMessage(mcp.RoleUser, mcp.NewTextContent(promptText))

		return mcp.NewGetPromptResult(
			"azqr scan subscription",
			[]mcp.PromptMessage{promptMessage},
		), nil
	}
}
