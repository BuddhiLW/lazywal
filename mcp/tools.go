// Copyright 2025 lazywal Pedro G. Branquinho
// SPDX-License-Identifier: MIT

package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/BuddhiLW/lazywal/loop"
	"github.com/mark3labs/mcp-go/mcp"
	Z "github.com/rwxrob/bonzai/z"
)

// MonitorInfo represents monitor information for JSON output
type MonitorInfo struct {
	Name    string  `json:"name"`
	Width   float32 `json:"width"`
	Height  float32 `json:"height"`
	X       float32 `json:"x"`
	Y       float32 `json:"y"`
	Primary bool    `json:"primary"`
}

// StatusInfo represents wallpaper status for JSON output
type StatusInfo struct {
	Path     string   `json:"path"`
	Running  bool     `json:"running"`
	PIDs     []int    `json:"pids"`
	Monitors []string `json:"monitors"`
}

// handleSetWallpaper sets a video/gif as wallpaper
func handleSetWallpaper(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	path, err := request.RequireString("path")
	if err != nil {
		return mcp.NewToolResultError("path parameter is required"), nil
	}

	// Validate path exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return mcp.NewToolResultError(fmt.Sprintf("file not found: %s", path)), nil
	}

	// Set the wallpaper path
	loop.Wall.Config.Path = path

	// Set wallpaper on all monitors
	if err := loop.Wall.Set(); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to set wallpaper: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Wallpaper set successfully: %s", path)), nil
}

// handleClearWallpaper kills all xwinwrap processes
func handleClearWallpaper(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	cmd := exec.Command("pkill", "xwinwrap")
	err := cmd.Run()

	// pkill returns exit code 1 if no processes found, which is fine
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if exitErr.ExitCode() == 1 {
				return mcp.NewToolResultText("No wallpaper processes were running"), nil
			}
		}
		return mcp.NewToolResultError(fmt.Sprintf("failed to clear wallpaper: %v", err)), nil
	}

	return mcp.NewToolResultText("Wallpaper cleared successfully"), nil
}

// handleListMonitors returns information about connected monitors
func handleListMonitors(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	monitors, err := loop.GetMonitors()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get monitors: %v", err)), nil
	}

	var monitorInfos []MonitorInfo
	for _, m := range monitors {
		monitorInfos = append(monitorInfos, MonitorInfo{
			Name:    m.Name,
			Width:   m.Dimensions.Width,
			Height:  m.Dimensions.Height,
			X:       m.Position.X,
			Y:       m.Position.Y,
			Primary: m.Primary,
		})
	}

	jsonBytes, err := json.MarshalIndent(monitorInfos, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal monitors: %v", err)), nil
	}

	return mcp.NewToolResultText(string(jsonBytes)), nil
}

// handleGetStatus returns the current wallpaper status
func handleGetStatus(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Initialize bonzai vars if needed
	if err := Z.Vars.SoftInit(); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to init vars: %v", err)), nil
	}

	// Get current path
	path := loop.Wall.Config.Path

	// Get PIDs from stored vars
	pidsStr := Z.Vars.Get(loop.VarPIDs)
	var pids []int
	if pidsStr != "" {
		for _, pidStr := range strings.Split(pidsStr, ",") {
			if pid, err := strconv.Atoi(pidStr); err == nil {
				pids = append(pids, pid)
			}
		}
	}

	// Get monitor names with running wallpapers
	var runningMonitors []string
	monitors, _ := loop.GetMonitors()
	for _, m := range monitors {
		monitorPidsStr := Z.Vars.Get(loop.VarMonitorPIDs + "_" + m.Name)
		if monitorPidsStr != "" {
			runningMonitors = append(runningMonitors, m.Name)
		}
	}

	status := StatusInfo{
		Path:     path,
		Running:  len(pids) > 0,
		PIDs:     pids,
		Monitors: runningMonitors,
	}

	jsonBytes, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal status: %v", err)), nil
	}

	return mcp.NewToolResultText(string(jsonBytes)), nil
}

// handleApplyPywal extracts a frame and applies pywal colors
func handleApplyPywal(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Check if wal is available
	if _, err := exec.LookPath("wal"); err != nil {
		return mcp.NewToolResultError("pywal (wal) is not installed or not in PATH"), nil
	}

	// Check if we have a wallpaper path set
	if loop.Wall.Config.Path == "" {
		return mcp.NewToolResultError("no wallpaper is currently set - use set_wallpaper first"), nil
	}

	// Apply pywal
	loop.Wall.Pywal()

	return mcp.NewToolResultText(fmt.Sprintf("Pywal colors applied from: %s", loop.Wall.Config.Path)), nil
}
