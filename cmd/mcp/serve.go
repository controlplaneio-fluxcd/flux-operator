// Copyright 2025 Stefan Prodan.
// SPDX-License-Identifier: AGPL-3.0

package main

import (
	mcpgolang "github.com/metoro-io/mcp-golang"
	"github.com/metoro-io/mcp-golang/transport/stdio"
	"github.com/spf13/cobra"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the MCP server",
	RunE:  serveCmdRun,
}

func init() {
	rootCmd.AddCommand(serveCmd)
}

func serveCmdRun(cmd *cobra.Command, args []string) error {
	done := make(chan struct{})

	mcpServer := mcpgolang.NewServer(stdio.NewStdioServerTransport())

	for _, tool := range ActionList {
		err := mcpServer.RegisterTool(tool.Name, tool.Description, tool.Handler)
		if err != nil {
			return err
		}
	}

	for _, tool := range ReportList {
		err := mcpServer.RegisterTool(tool.Name, tool.Description, tool.Handler)
		if err != nil {
			return err
		}
	}

	for _, resource := range DocumentationList {
		err := mcpServer.RegisterResource(
			resource.Path,
			resource.Name,
			resource.Description,
			resource.ContentType,
			resource.Handler)
		if err != nil {
			return err
		}
	}

	err := mcpServer.Serve()
	if err != nil {
		return err
	}

	<-done

	return nil
}
