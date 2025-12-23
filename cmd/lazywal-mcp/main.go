// Copyright 2025 lazywal Pedro G. Branquinho
// SPDX-License-Identifier: MIT

package main

import (
	"log"

	lazywalmcp "github.com/BuddhiLW/lazywal/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func main() {
	// Create the MCP server
	s := lazywalmcp.NewServer()

	// Start the server with stdio transport
	if err := server.ServeStdio(s); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
