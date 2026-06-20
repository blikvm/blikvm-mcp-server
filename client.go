package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client 是 blikvm web server 的 REST API 客户端。
// 它负责登录认证并调用截图、键鼠等接口。
type Client struct {
	baseURL  string
	username string
	password string
	token    string
	http     *http.Client
}

// NewClient 创建一个新的 blikvm API 客户端。
func NewClient(baseURL, username, password string) *Client {
	return &Client{
		baseURL:  baseURL,
		username: username,
		password: password,
		http: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// loginResponse 是 /api/v1/auth/login 的响应结构。
type loginResponse struct {
	OK   bool `json:"ok"`
	Data struct {
		Token     string `json:"token"`
		ExpiresAt string `json:"expiresAt"`
	} `json:"data"`
	Error string `json:"error"`
}

// Login 使用用户名密码登录 blikvm，获取 Bearer token。
func (c *Client) Login(ctx context.Context) error {
	payload, _ := json.Marshal(map[string]string{
		"username": c.username,
		"password": c.password,
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/v1/auth/login", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("login request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var lr loginResponse
	if err := json.Unmarshal(body, &lr); err != nil {
		return fmt.Errorf("login response parse failed: %w", err)
	}
	if !lr.OK || lr.Data.Token == "" {
		return fmt.Errorf("login failed: %s", lr.Error)
	}

	c.token = lr.Data.Token
	return nil
}

// ensureToken 确保客户端持有有效 token，若没有则自动登录。
func (c *Client) ensureToken(ctx context.Context) error {
	if c.token != "" {
		return nil
	}
	return c.Login(ctx)
}

// doRequest 发送带认证的 HTTP 请求，遇到 401 时自动重新登录重试一次。
func (c *Client) doRequest(ctx context.Context, method, path string, body io.Reader) (*http.Response, error) {
	if err := c.ensureToken(ctx); err != nil {
		return nil, err
	}

	doOnce := func() (*http.Response, error) {
		req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+c.token)
		return c.http.Do(req)
	}

	resp, err := doOnce()
	if err != nil {
		return nil, err
	}

	// 401 -> 重新登录后重试一次
	if resp.StatusCode == http.StatusUnauthorized {
		resp.Body.Close()
		c.token = ""
		if err := c.Login(ctx); err != nil {
			return nil, err
		}
		return doOnce()
	}
	return resp, nil
}

// TakeSnapshot 调用 /api/v1/video/snapshot 获取当前画面的 JPEG 截图。
func (c *Client) TakeSnapshot(ctx context.Context) ([]byte, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, "/api/v1/video/snapshot", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("snapshot failed: HTTP %d: %s", resp.StatusCode, string(body))
	}

	return io.ReadAll(resp.Body)
}

// hidEventPayload 是 /api/v1/hid/events 的请求体。
type hidEventPayload struct {
	Type   string   `json:"type"`
	Key    string   `json:"key,omitempty"`
	State  *bool    `json:"state,omitempty"`
	Finish bool     `json:"finish,omitempty"`
	Button string   `json:"button,omitempty"`
	ToX    *float64 `json:"toX,omitempty"`
	ToY    *float64 `json:"toY,omitempty"`
	DeltaX *float64 `json:"deltaX,omitempty"`
	DeltaY *float64 `json:"deltaY,omitempty"`
}

// sendHidEvent 发送一个 HID 事件到 blikvm。
func (c *Client) sendHidEvent(ctx context.Context, payload hidEventPayload) error {
	body, _ := json.Marshal(payload)
	resp, err := c.doRequest(ctx, http.MethodPost, "/api/v1/hid/events", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("hid event failed: HTTP %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}

// MouseMove 将鼠标移动到归一化绝对坐标 (x, y ∈ [0, 1])。
func (c *Client) MouseMove(ctx context.Context, x, y float64) error {
	return c.sendHidEvent(ctx, hidEventPayload{
		Type: "mouseMove",
		ToX:  &x,
		ToY:  &y,
	})
}

// MouseButton 按下或释放鼠标按钮。button: left/right/middle。
func (c *Client) MouseButton(ctx context.Context, button string, pressed bool) error {
	return c.sendHidEvent(ctx, hidEventPayload{
		Type:   "mouseButton",
		Button: button,
		State:  &pressed,
	})
}

// MouseWheel 滚动鼠标滚轮。
func (c *Client) MouseWheel(ctx context.Context, deltaX, deltaY float64) error {
	return c.sendHidEvent(ctx, hidEventPayload{
		Type:   "mouseWheel",
		DeltaX: &deltaX,
		DeltaY: &deltaY,
	})
}

// KeyTap 按下并释放一个键盘键。key 例如 "Enter", "Escape", "A"。
func (c *Client) KeyTap(ctx context.Context, key string) error {
	return c.sendHidEvent(ctx, hidEventPayload{
		Type:   "keyboard",
		Key:    key,
		Finish: true,
	})
}

// KeyPress 按下或释放一个键盘键。
func (c *Client) KeyPress(ctx context.Context, key string, pressed bool) error {
	return c.sendHidEvent(ctx, hidEventPayload{
		Type:  "keyboard",
		Key:   key,
		State: &pressed,
	})
}

// pasteRequest 是 /api/v1/hid/paste 的请求体。
type pasteRequest struct {
	Sequence [][]string `json:"sequence"`
}

// TypeText 通过 paste API 输入文本。
func (c *Client) TypeText(ctx context.Context, text string) error {
	// paste API 的 sequence 是 [][]string，每个子数组是一组同时按下的键。
	// 对于普通文本输入，我们把每个字符作为一个独立的步骤。
	// 但 paste API 实际上接受的是 HID 键码序列，不是直接文本。
	// 这里我们使用一个简化方案：将文本拆成单字符序列。
	// 注意：blikvm 的 paste API 期望的是 HID key code，不是字面字符。
	// 对于纯 ASCII 可打印字符，我们使用对应的 key code。
	sequence := buildPasteSequence(text)
	if len(sequence) == 0 {
		return fmt.Errorf("empty text")
	}

	payload, _ := json.Marshal(pasteRequest{Sequence: sequence})
	resp, err := c.doRequest(ctx, http.MethodPost, "/api/v1/hid/paste", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("type text failed: HTTP %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

// buildPasteSequence 将文本转换为 paste API 可接受的 HID 键码序列。
// 对于可打印 ASCII 字符，直接使用字符本身作为 key code（blikvm 支持单字符 key）。
// 对于需要 Shift 的符号（如 !@#$%^&*()），使用 ["Shift", 字符] 组合。
func buildPasteSequence(text string) [][]string {
	var seq [][]string
	for _, r := range text {
		s := string(r)
		if shiftKeys, ok := shiftMap[s]; ok {
			seq = append(seq, shiftKeys)
		} else {
			seq = append(seq, []string{s})
		}
	}
	return seq
}

// shiftMap 映射需要 Shift 键的符号到键码组合。
var shiftMap = map[string][]string{
	"!": {"Shift", "1"},
	"@": {"Shift", "2"},
	"#": {"Shift", "3"},
	"$": {"Shift", "4"},
	"%": {"Shift", "5"},
	"^": {"Shift", "6"},
	"&": {"Shift", "7"},
	"*": {"Shift", "8"},
	"(": {"Shift", "9"},
	")": {"Shift", "0"},
	"_": {"Shift", "-"},
	"+": {"Shift", "="},
	"{": {"Shift", "["},
	"}": {"Shift", "]"},
	"|": {"Shift", "\\"},
	":": {"Shift", ";"},
	"\"": {"Shift", "'"},
	"<": {"Shift", ","},
	">": {"Shift", "."},
	"?": {"Shift", "/"},
	"~": {"Shift", "`"},
}
