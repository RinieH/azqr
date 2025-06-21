// Copyright (c) Microsoft Corporation.
// Licensed under the MIT License.

package mcpserver

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/Azure/azqr/internal"
	"github.com/Azure/azqr/internal/models"
	"github.com/Azure/azqr/internal/renderers"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

type ScanArgs struct {
	Services []string `json:"services,omitempty"`
	Defender *bool    `json:"defender,omitempty"`
	Advisor  *bool    `json:"advisor,omitempty"`
	Cost     *bool    `json:"cost,omitempty"`
	Policy   *bool    `json:"policy,omitempty"`
	Arc      *bool    `json:"arc,omitempty"`
	Mask     *bool    `json:"mask,omitempty"`
}

func RegsiterTools(s *server.MCPServer) {
	// Create a new resource to get the current working directory
	currentFolder := mcp.NewTool("current-folder",
		mcp.WithDescription("Returns the current working directory of the server process."),
	)

	// Tool: Get current working directory
	s.AddTool(currentFolder, currentFolderHandler)

	scan := mcp.NewTool("scan",
		mcp.WithDescription(
			`Run an Azure Quick Review (azqr) scan to analyze Azure resources and identify recommendations for improvement.

			WHAT GETS SCANNED (by default):
			- Azure resources and their configurations (Well-Architected Framework recommendations)
			- Microsoft Defender for Cloud security posture and recommendations
			- Azure Advisor recommendations (cost optimization, performance, reliability, operational excellence)
			- Cost analysis (last 3 months of costs by resource)
			- Resource inventory and metadata
			- Best practice violations and configuration issues

			SERVICE SCOPE:
			- If services parameter is NOT provided or is EMPTY [] -> Comprehensive scan of ALL supported Azure resource types
			- If services parameter contains specific service abbreviations -> Only those service types will be scanned

			Examples:
			- {} or {"services": []} -> Full comprehensive scan (all resources, costs, advisor, defender)
			- {"services": ["st"]} -> Scan only Storage Accounts (plus costs, advisor, defender)
			- {"services": ["aks", "vm", "sql"], "cost": false} -> Scan AKS/VM/SQL without cost analysis
			- {"defender": false, "advisor": false} -> Scan all resources but skip Defender and Advisor

			Use get-supported-services tool to see all available service abbreviations.`),
		mcp.WithArray("services",
			mcp.Items(map[string]any{"type": "string"}),
			mcp.Description("Optional array of service type abbreviations to scan (e.g., ['aks', 'st', 'sql']). Leave empty or omit to scan all supported resource types. Use get-supported-services tool to see available abbreviations."),
		),
		mcp.WithBoolean("defender",
			mcp.Description("Include Microsoft Defender for Cloud scanning (default: true). Set to false to skip Defender scanning."),
		),
		mcp.WithBoolean("advisor",
			mcp.Description("Include Azure Advisor recommendations (default: true). Set to false to skip Advisor scanning."),
		),
		mcp.WithBoolean("cost",
			mcp.Description("Include cost analysis (default: true). Set to false to skip cost analysis. Useful if you have permission issues."),
		),
		mcp.WithBoolean("policy",
			mcp.Description("Include Azure Policy compliance scanning (default: false). Set to true to enable policy scanning."),
		),
		mcp.WithBoolean("arc",
			mcp.Description("Include Arc-enabled SQL Server scanning (default: false). Set to true to enable Arc SQL scanning."),
		),
		mcp.WithBoolean("mask",
			mcp.Description("Mask sensitive data in output (default: true). Set to false to include full resource details."),
		),
	)

	// Tool: Scan
	s.AddTool(scan, mcp.NewTypedToolHandler(scanHandler))

	// Tool: Get recommendation catalog
	catalogTool := mcp.NewTool("get-recommendations-catalog",
		mcp.WithDescription("Get the complete catalog of AZQR recommendations"),
	)

	s.AddTool(catalogTool, handleCatalogTool)

	// Tool: Get supported services
	servicesTool := mcp.NewTool("get-supported-services",
		mcp.WithDescription("Get the list of Azure services supported by AZQR"),
	)

	s.AddTool(servicesTool, handleServiceTypeTool)
}

func handleServiceTypeTool(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	st := renderers.SupportedTypes{}
	output := st.GetAll()

	return mcp.NewToolResultText(output), nil
}

func handleCatalogTool(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	output := renderers.GetAllRecommendations(true)

	jsonBytes, _ := json.MarshalIndent(output, "", "  ")

	return mcp.NewToolResultText(string(jsonBytes)), nil
}

func currentFolderHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	currentDir, err := getCurrentFolder()
	if err != nil {
		return nil, err
	}

	return mcp.NewToolResultText(currentDir), nil
}

func scanHandler(ctx context.Context, request mcp.CallToolRequest, args ScanArgs) (*mcp.CallToolResult, error) {
	currentDir, err := getCurrentFolder()
	if err != nil {
		log.Fatal(fmt.Errorf("failed to get current working directory: %w", err))
	}

	scannerKeys := args.Services
	filters := models.LoadFilters("", scannerKeys)
	params := internal.NewScanParams()

	// Override defaults with provided values
	if args.Defender != nil {
		params.Defender = *args.Defender
	}
	if args.Advisor != nil {
		params.Advisor = *args.Advisor
	}
	if args.Cost != nil {
		params.Cost = *args.Cost
	}
	if args.Policy != nil {
		params.Policy = *args.Policy
	}
	if args.Arc != nil {
		params.Arc = *args.Arc
	}
	if args.Mask != nil {
		params.Mask = *args.Mask
	}

	params.Xlsx = true
	params.Json = true
	params.ScannerKeys = scannerKeys
	params.Filters = filters
	params.OutputName = currentDir + "/azqr_scan_results"
	scanner := internal.Scanner{}
	r := scanner.Scan(params)

	fileName := params.OutputName + ".xlsx"
	uri := fmt.Sprintf("file://%s", fileName)
	uriJSON := fmt.Sprintf("file://%s.json", params.OutputName)

	// Register the scan results as a resource
	jsonResults := mcp.NewResource(
		uriJSON,
		"Azure Quick Review Scan Results Metadata",
		mcp.WithResourceDescription(`The metadata of the Azure Quick Review (azqr) scan for the specified resource type.`),
		mcp.WithMIMEType("application/json"),
	)

	jsonBlob, err := os.ReadFile(params.OutputName + ".json")
	if err != nil {
		log.Fatal(fmt.Errorf("failed to read scan results metadata file: %w", err))
	}

	encodedJSONBlob := make([]byte, base64.StdEncoding.EncodedLen(len(jsonBlob)))
	base64.StdEncoding.Encode(encodedJSONBlob, jsonBlob)

	s.AddResource(jsonResults, func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		return []mcp.ResourceContents{
			mcp.BlobResourceContents{
				URI:      uriJSON,
				MIMEType: "application/json",
				Blob:     string(encodedJSONBlob),
			},
		}, nil
	})

	results := mcp.NewResource(
		uri,
		"Azure Quick Review Scan Results",
		mcp.WithResourceDescription(`The results of the Azure Quick Review (azqr) scan for the specified resource type.`),
		mcp.WithMIMEType("binary/octet-stream"),
	)

	fileBlob, err := os.ReadFile(fileName)
	if err != nil {
		log.Fatal(fmt.Errorf("failed to read scan results file: %w", err))
	}

	encodedBlob := make([]byte, base64.StdEncoding.EncodedLen(len(fileBlob)))
	base64.StdEncoding.Encode(encodedBlob, fileBlob)

	s.AddResource(results, func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		return []mcp.ResourceContents{
			mcp.BlobResourceContents{
				URI:      uri,
				MIMEType: "binary/octet-stream",
				Blob:     string(encodedBlob),
			},
		}, nil
	})

	// Notify client that new resources are available using standard MCP notification
	s.SendNotificationToClient(context.Background(),
		"notifications/resources/list_changed",
		nil,
	)

	// Return both the scan results and the resource URIs
	resultText := fmt.Sprintf("%s\n\nScan results saved to:\n- Excel: %s\n- JSON: %s", r, uri, uriJSON)
	return mcp.NewToolResultText(resultText), nil
}

func getCurrentFolder() (string, error) {
	currentDir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current working directory: %w", err)
	}
	return currentDir, nil
}
