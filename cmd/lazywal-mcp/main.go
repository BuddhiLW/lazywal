// Copyright 2025 lazywal Pedro G. Branquinho
// SPDX-License-Identifier: MIT

package main

import (
	"log"

	"github.com/BuddhiLW/lazywal/loop"
	"github.com/mark3labs/mcp-go/server"
	bmcp "github.com/rwxrob/bonzai/mcp"
)

func main() {
	// Create the MCP server from Bonzai command tree
	// OnlyTagged() ensures only commands with Mcp metadata are exposed
	s := bmcp.NewServer(loop.Cmd, bmcp.OnlyTagged())

	// Start the server with stdio transport
	if err := server.ServeStdio(s); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
