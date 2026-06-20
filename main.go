package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/mark3labs/mcp-go/server"
)

// blikvm-mcp-server 是一个 MCP (Model Context Protocol) 服务器，
// 它将 BliKVM 的截图和键鼠控制能力暴露为 MCP 工具，
// 让 AI agent 可以通过 MCP 协议远程控制被控 PC。
//
// 用法:
//
//	blikvm-mcp-server -url http://<blikvm-ip> -username admin -password <password>
//
// 环境变量（优先级低于命令行参数）:
//
//	BLIKVM_URL      blikvm web server 地址
//	BLIKVM_USERNAME 登录用户名
//	BLIKVM_PASSWORD 登录密码
func main() {
	url := flag.String("url", "", "BliKVM web server URL (e.g. http://192.168.1.100)")
	username := flag.String("username", "", "BliKVM login username")
	password := flag.String("password", "", "BliKVM login password")
	flag.Parse()

	// 命令行参数优先，环境变量作为后备
	if *url == "" {
		*url = os.Getenv("BLIKVM_URL")
	}
	if *username == "" {
		*username = os.Getenv("BLIKVM_USERNAME")
	}
	if *password == "" {
		*password = os.Getenv("BLIKVM_PASSWORD")
	}

	if *url == "" || *username == "" || *password == "" {
		fmt.Fprintln(os.Stderr, "Error: --url, --username, and --password are required")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Usage:")
		fmt.Fprintln(os.Stderr, "  blikvm-mcp-server -url http://<blikvm-ip> -username <user> -password <pass>")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Or set environment variables:")
		fmt.Fprintln(os.Stderr, "  BLIKVM_URL, BLIKVM_USERNAME, BLIKVM_PASSWORD")
		os.Exit(1)
	}

	// 创建 blikvm API 客户端
	client := NewClient(*url, *username, *password)

	// 创建 MCP server
	mcpServer := server.NewMCPServer(
		"blikvm-mcp-server",
		"1.0.0",
		server.WithToolCapabilities(true),
	)

	// 注册所有工具
	registerTools(mcpServer, client)

	// 通过 stdio 启动 MCP server（标准 MCP 传输方式）
	stdioServer := server.NewStdioServer(mcpServer)
	if err := stdioServer.Listen(context.Background(), os.Stdin, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "MCP server error: %v\n", err)
		os.Exit(1)
	}
}
