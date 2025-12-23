// Copyright 2025 lazywal Pedro G. Branquinho
// SPDX-License-Identifier: MIT

package mcp

import (
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// NewServer creates and configures the lazywal MCP server
func NewServer() *server.MCPServer {
	s := server.NewMCPServer(
		"lazywal",
		"1.0.0",
		server.WithToolCapabilities(true),
	)

	// Register all tools
	registerTools(s)

	return s
}

// registerTools adds all lazywal tools to the server
func registerTools(s *server.MCPServer) {
	// set_wallpaper - Set a video/gif as wallpaper
	s.AddTool(
		mcp.NewTool("set_wallpaper",
			mcp.WithDescription("Set a video or animated GIF as wallpaper on all monitors"),
			mcp.WithString("path",
				mcp.Required(),
				mcp.Description("Absolute path to the video or GIF file"),
			),
		),
		handleSetWallpaper,
	)

	// clear_wallpaper - Kill all xwinwrap processes
	s.AddTool(
		mcp.NewTool("clear_wallpaper",
			mcp.WithDescription("Clear the wallpaper by killing all xwinwrap processes"),
		),
		handleClearWallpaper,
	)

	// list_monitors - Get connected monitors info
	s.AddTool(
		mcp.NewTool("list_monitors",
			mcp.WithDescription("List all connected monitors with their dimensions and positions"),
		),
		handleListMonitors,
	)

	// get_status - Get current wallpaper status
	s.AddTool(
		mcp.NewTool("get_status",
			mcp.WithDescription("Get the current wallpaper status including path and running processes"),
		),
		handleGetStatus,
	)

	// apply_pywal - Extract frame and apply pywal colors
	s.AddTool(
		mcp.NewTool("apply_pywal",
			mcp.WithDescription("Extract a random frame from the current wallpaper and apply pywal color scheme"),
		),
		handleApplyPywal,
	)
}
