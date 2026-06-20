package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// registerTools 注册所有 MCP 工具到给定的 MCP server。
func registerTools(s *server.MCPServer, client *Client) {
	registerScreenshot(s, client)
	registerMouseMove(s, client)
	registerMouseClick(s, client)
	registerMouseDoubleClick(s, client)
	registerMouseDrag(s, client)
	registerMouseScroll(s, client)
	registerKeyTap(s, client)
	registerKeyHotkey(s, client)
	registerTypeText(s, client)
}

// getArg 从 CallToolRequest 的 Arguments 中安全提取参数。
func getArg(req mcp.CallToolRequest, key string) (any, bool) {
	args := req.GetArguments()
	if args == nil {
		return nil, false
	}
	v, ok := args[key]
	return v, ok
}

// ============================================================================
// screenshot 工具
// ============================================================================

func registerScreenshot(s *server.MCPServer, client *Client) {
	tool := mcp.NewTool("blikvm_screenshot",
		mcp.WithDescription("Take a screenshot of the remote screen controlled by BliKVM. "+
			"Returns the screenshot as a base64-encoded JPEG image. "+
			"Use this to see the current state of the remote display before performing mouse or keyboard actions."),
	)
	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		jpeg, err := client.TakeSnapshot(ctx)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("screenshot failed: %v", err)), nil
		}
		b64 := base64.StdEncoding.EncodeToString(jpeg)
		return mcp.NewToolResultImage("Screenshot captured", b64, "image/jpeg"), nil
	})
}

// ============================================================================
// mouse_move 工具
// ============================================================================

func registerMouseMove(s *server.MCPServer, client *Client) {
	tool := mcp.NewTool("blikvm_mouse_move",
		mcp.WithDescription("Move the mouse cursor to an absolute position on the remote screen. "+
			"Coordinates are normalized to [0.0, 1.0] where (0,0) is top-left and (1,1) is bottom-right. "+
			"Estimate the position based on the screenshot you took."),
		mcp.WithNumber("x", mcp.Required(), mcp.Description("Normalized X coordinate [0.0 - 1.0]"), mcp.Min(0), mcp.Max(1)),
		mcp.WithNumber("y", mcp.Required(), mcp.Description("Normalized Y coordinate [0.0 - 1.0]"), mcp.Min(0), mcp.Max(1)),
	)
	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		x, ok := getArg(req, "x")
		if !ok {
			return mcp.NewToolResultError("missing 'x' parameter"), nil
		}
		y, ok := getArg(req, "y")
		if !ok {
			return mcp.NewToolResultError("missing 'y' parameter"), nil
		}
		xf, ok := x.(float64)
		if !ok {
			return mcp.NewToolResultError("invalid 'x' parameter"), nil
		}
		yf, ok := y.(float64)
		if !ok {
			return mcp.NewToolResultError("invalid 'y' parameter"), nil
		}
		if err := client.MouseMove(ctx, xf, yf); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("mouse move failed: %v", err)), nil
		}
		return mcp.NewToolResultText(fmt.Sprintf("Mouse moved to (%.4f, %.4f)", xf, yf)), nil
	})
}

// ============================================================================
// mouse_click 工具
// ============================================================================

func registerMouseClick(s *server.MCPServer, client *Client) {
	tool := mcp.NewTool("blikvm_mouse_click",
		mcp.WithDescription("Click the mouse at the current cursor position. "+
			"Optionally move to a position first by providing x and y. "+
			"Use blikvm_mouse_move to move the cursor first if you only need to click at current position."),
		mcp.WithNumber("x", mcp.Description("Optional: move to this normalized X [0.0-1.0] before clicking")),
		mcp.WithNumber("y", mcp.Description("Optional: move to this normalized Y [0.0-1.0] before clicking")),
		mcp.WithString("button", mcp.Description("Mouse button: left (default), right, middle"),
			mcp.Enum("left", "right", "middle")),
	)
	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		button := "left"
		if b, ok := getArg(req, "button"); ok {
			if bs, ok := b.(string); ok && bs != "" {
				button = bs
			}
		}

		// 可选：先移动
		if x, ok := getArg(req, "x"); ok {
			if y, ok2 := getArg(req, "y"); ok2 {
				if xf, ok3 := x.(float64); ok3 {
					if yf, ok4 := y.(float64); ok4 {
						if err := client.MouseMove(ctx, xf, yf); err != nil {
							return mcp.NewToolResultError(fmt.Sprintf("mouse move failed: %v", err)), nil
						}
						time.Sleep(50 * time.Millisecond)
					}
				}
			}
		}

		// 按下并释放
		if err := client.MouseButton(ctx, button, true); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("mouse press failed: %v", err)), nil
		}
		time.Sleep(40 * time.Millisecond)
		if err := client.MouseButton(ctx, button, false); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("mouse release failed: %v", err)), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("Clicked %s button", button)), nil
	})
}

// ============================================================================
// mouse_double_click 工具
// ============================================================================

func registerMouseDoubleClick(s *server.MCPServer, client *Client) {
	tool := mcp.NewTool("blikvm_mouse_double_click",
		mcp.WithDescription("Double-click the mouse at the current cursor position. "+
			"Optionally move to a position first by providing x and y."),
		mcp.WithNumber("x", mcp.Description("Optional: move to this normalized X [0.0-1.0] before clicking")),
		mcp.WithNumber("y", mcp.Description("Optional: move to this normalized Y [0.0-1.0] before clicking")),
		mcp.WithString("button", mcp.Description("Mouse button: left (default), right, middle"),
			mcp.Enum("left", "right", "middle")),
	)
	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		button := "left"
		if b, ok := getArg(req, "button"); ok {
			if bs, ok := b.(string); ok && bs != "" {
				button = bs
			}
		}

		if x, ok := getArg(req, "x"); ok {
			if y, ok2 := getArg(req, "y"); ok2 {
				if xf, ok3 := x.(float64); ok3 {
					if yf, ok4 := y.(float64); ok4 {
						if err := client.MouseMove(ctx, xf, yf); err != nil {
							return mcp.NewToolResultError(fmt.Sprintf("mouse move failed: %v", err)), nil
						}
						time.Sleep(50 * time.Millisecond)
					}
				}
			}
		}

		// 两次点击
		for i := 0; i < 2; i++ {
			if err := client.MouseButton(ctx, button, true); err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("mouse press failed: %v", err)), nil
			}
			time.Sleep(40 * time.Millisecond)
			if err := client.MouseButton(ctx, button, false); err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("mouse release failed: %v", err)), nil
			}
			if i == 0 {
				time.Sleep(80 * time.Millisecond)
			}
		}

		return mcp.NewToolResultText(fmt.Sprintf("Double-clicked %s button", button)), nil
	})
}

// ============================================================================
// mouse_drag 工具
// ============================================================================

func registerMouseDrag(s *server.MCPServer, client *Client) {
	tool := mcp.NewTool("blikvm_mouse_drag",
		mcp.WithDescription("Drag the mouse from one position to another (hold left button while moving). "+
			"All coordinates are normalized [0.0-1.0]."),
		mcp.WithNumber("fromX", mcp.Required(), mcp.Description("Start normalized X [0.0-1.0]")),
		mcp.WithNumber("fromY", mcp.Required(), mcp.Description("Start normalized Y [0.0-1.0]")),
		mcp.WithNumber("toX", mcp.Required(), mcp.Description("End normalized X [0.0-1.0]")),
		mcp.WithNumber("toY", mcp.Required(), mcp.Description("End normalized Y [0.0-1.0]")),
	)
	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		getFloat := func(key string) (float64, bool) {
			v, ok := getArg(req, key)
			if !ok {
				return 0, false
			}
			f, ok := v.(float64)
			return f, ok
		}

		fromX, ok := getFloat("fromX")
		if !ok {
			return mcp.NewToolResultError("missing 'fromX'"), nil
		}
		fromY, ok := getFloat("fromY")
		if !ok {
			return mcp.NewToolResultError("missing 'fromY'"), nil
		}
		toX, ok := getFloat("toX")
		if !ok {
			return mcp.NewToolResultError("missing 'toX'"), nil
		}
		toY, ok := getFloat("toY")
		if !ok {
			return mcp.NewToolResultError("missing 'toY'"), nil
		}

		// 移动到起点
		if err := client.MouseMove(ctx, fromX, fromY); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("move to start failed: %v", err)), nil
		}
		time.Sleep(100 * time.Millisecond)

		// 按下左键
		if err := client.MouseButton(ctx, "left", true); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("press failed: %v", err)), nil
		}
		time.Sleep(50 * time.Millisecond)

		// 移动到终点
		if err := client.MouseMove(ctx, toX, toY); err != nil {
			_ = client.MouseButton(ctx, "left", false)
			return mcp.NewToolResultError(fmt.Sprintf("move to end failed: %v", err)), nil
		}
		time.Sleep(50 * time.Millisecond)

		// 释放
		if err := client.MouseButton(ctx, "left", false); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("release failed: %v", err)), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("Dragged from (%.4f,%.4f) to (%.4f,%.4f)", fromX, fromY, toX, toY)), nil
	})
}

// ============================================================================
// mouse_scroll 工具
// ============================================================================

func registerMouseScroll(s *server.MCPServer, client *Client) {
	tool := mcp.NewTool("blikvm_mouse_scroll",
		mcp.WithDescription("Scroll the mouse wheel at the current cursor position. "+
			"Positive deltaY scrolls down, negative scrolls up. "+
			"Typical values: ±1.0 for one wheel notch, ±5.0 for fast scroll."),
		mcp.WithNumber("deltaY", mcp.Required(), mcp.Description("Vertical scroll amount. Positive=down, negative=up")),
	)
	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		v, ok := getArg(req, "deltaY")
		if !ok {
			return mcp.NewToolResultError("missing 'deltaY'"), nil
		}
		deltaY, ok := v.(float64)
		if !ok {
			return mcp.NewToolResultError("invalid 'deltaY'"), nil
		}
		if err := client.MouseWheel(ctx, 0, deltaY); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("scroll failed: %v", err)), nil
		}
		return mcp.NewToolResultText(fmt.Sprintf("Scrolled deltaY=%.2f", deltaY)), nil
	})
}

// ============================================================================
// key_tap 工具
// ============================================================================

func registerKeyTap(s *server.MCPServer, client *Client) {
	tool := mcp.NewTool("blikvm_key_tap",
		mcp.WithDescription("Tap (press and release) a single keyboard key on the remote machine. "+
			"Common keys: Enter, Escape, Tab, Space, Backspace, Delete, Up, Down, Left, Right, "+
			"Home, End, PageUp, PageDown, F1-F12, CapsLock, NumLock. "+
			"For letters/numbers, use the character directly (e.g. \"A\", \"1\")."),
		mcp.WithString("key", mcp.Required(), mcp.Description("Key name, e.g. \"Enter\", \"Escape\", \"A\"")),
	)
	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		v, ok := getArg(req, "key")
		if !ok {
			return mcp.NewToolResultError("missing 'key'"), nil
		}
		key, ok := v.(string)
		if !ok || key == "" {
			return mcp.NewToolResultError("missing or invalid 'key'"), nil
		}
		if err := client.KeyTap(ctx, key); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("key tap failed: %v", err)), nil
		}
		return mcp.NewToolResultText(fmt.Sprintf("Tapped key: %s", key)), nil
	})
}

// ============================================================================
// key_hotkey 工具
// ============================================================================

func registerKeyHotkey(s *server.MCPServer, client *Client) {
	tool := mcp.NewTool("blikvm_key_hotkey",
		mcp.WithDescription("Press a keyboard shortcut (multiple keys held simultaneously then released). "+
			"Example: [\"Control\", \"C\"] for Ctrl+C, [\"Control\", \"Shift\", \"V\"] for Ctrl+Shift+V. "+
			"Common modifier keys: Control, Shift, Alt, GUI (Windows/Command)."),
		mcp.WithArray("keys", mcp.Required(), mcp.Description("Array of key names to press together, e.g. [\"Control\", \"C\"]"),
			mcp.Items(map[string]any{"type": "string"})),
	)
	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		v, ok := getArg(req, "keys")
		if !ok {
			return mcp.NewToolResultError("missing 'keys' array"), nil
		}
		keysRaw, ok := v.([]any)
		if !ok || len(keysRaw) == 0 {
			return mcp.NewToolResultError("'keys' must be a non-empty array"), nil
		}

		var keys []string
		for _, k := range keysRaw {
			if s, ok := k.(string); ok && s != "" {
				keys = append(keys, s)
			}
		}
		if len(keys) == 0 {
			return mcp.NewToolResultError("no valid keys provided"), nil
		}

		// 依次按下所有键
		for _, k := range keys {
			if err := client.KeyPress(ctx, k, true); err != nil {
				// 释放已按下的键
				for _, pressed := range keys {
					_ = client.KeyPress(ctx, pressed, false)
				}
				return mcp.NewToolResultError(fmt.Sprintf("press key %s failed: %v", k, err)), nil
			}
			time.Sleep(20 * time.Millisecond)
		}

		time.Sleep(50 * time.Millisecond)

		// 反向释放所有键
		for i := len(keys) - 1; i >= 0; i-- {
			if err := client.KeyPress(ctx, keys[i], false); err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("release key %s failed: %v", keys[i], err)), nil
			}
			time.Sleep(20 * time.Millisecond)
		}

		return mcp.NewToolResultText(fmt.Sprintf("Pressed hotkey: %v", keys)), nil
	})
}

// ============================================================================
// type_text 工具
// ============================================================================

func registerTypeText(s *server.MCPServer, client *Client) {
	tool := mcp.NewTool("blikvm_type_text",
		mcp.WithDescription("Type a text string on the remote machine via the keyboard. "+
			"Use this to enter text into input fields, search boxes, terminals, etc. "+
			"For special keys (Enter, Tab, etc.), use blikvm_key_tap instead."),
		mcp.WithString("text", mcp.Required(), mcp.Description("The text to type")),
	)
	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		v, ok := getArg(req, "text")
		if !ok {
			return mcp.NewToolResultError("missing 'text'"), nil
		}
		text, ok := v.(string)
		if !ok || text == "" {
			return mcp.NewToolResultError("missing or empty 'text'"), nil
		}
		if err := client.TypeText(ctx, text); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("type text failed: %v", err)), nil
		}
		return mcp.NewToolResultText(fmt.Sprintf("Typed %d characters", len(text))), nil
	})
}
