package main

import (
	"bytes"
	"context"
	"crypto/tls"
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
// 注意：跳过 TLS 证书验证，适用于内部网络环境。
func NewClient(baseURL, username, password string) *Client {
	return &Client{
		baseURL:  baseURL,
		username: username,
		password: password,
		http: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true, // 跳过证书验证
				},
			},
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

// Hotkey 使用 paste API 同时按下一组键（组合键）。
// keys 为要同时按下的 HID 键码列表，如 ["ControlLeft", "KeyA"]。
func (c *Client) Hotkey(ctx context.Context, keys []string) error {
	if len(keys) == 0 {
		return fmt.Errorf("empty keys")
	}
	payload, _ := json.Marshal(pasteRequest{Sequence: [][]string{keys}})
	resp, err := c.doRequest(ctx, http.MethodPost, "/api/v1/hid/paste", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("hotkey failed: HTTP %d: %s", resp.StatusCode, string(body))
	}
	return nil
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

// charToHIDKey 将单个字符映射到 blikvm 支持的标准 HID 键码。
// 返回键码和是否需要 Shift。
var charToHIDKey = map[string]struct {
	code  string
	shift bool
}{
	"a": {"KeyA", false}, "b": {"KeyB", false}, "c": {"KeyC", false},
	"d": {"KeyD", false}, "e": {"KeyE", false}, "f": {"KeyF", false},
	"g": {"KeyG", false}, "h": {"KeyH", false}, "i": {"KeyI", false},
	"j": {"KeyJ", false}, "k": {"KeyK", false}, "l": {"KeyL", false},
	"m": {"KeyM", false}, "n": {"KeyN", false}, "o": {"KeyO", false},
	"p": {"KeyP", false}, "q": {"KeyQ", false}, "r": {"KeyR", false},
	"s": {"KeyS", false}, "t": {"KeyT", false}, "u": {"KeyU", false},
	"v": {"KeyV", false}, "w": {"KeyW", false}, "x": {"KeyX", false},
	"y": {"KeyY", false}, "z": {"KeyZ", false},
	"A": {"KeyA", true}, "B": {"KeyB", true}, "C": {"KeyC", true},
	"D": {"KeyD", true}, "E": {"KeyE", true}, "F": {"KeyF", true},
	"G": {"KeyG", true}, "H": {"KeyH", true}, "I": {"KeyI", true},
	"J": {"KeyJ", true}, "K": {"KeyK", true}, "L": {"KeyL", true},
	"M": {"KeyM", true}, "N": {"KeyN", true}, "O": {"KeyO", true},
	"P": {"KeyP", true}, "Q": {"KeyQ", true}, "R": {"KeyR", true},
	"S": {"KeyS", true}, "T": {"KeyT", true}, "U": {"KeyU", true},
	"V": {"KeyV", true}, "W": {"KeyW", true}, "X": {"KeyX", true},
	"Y": {"KeyY", true}, "Z": {"KeyZ", true},
	"1": {"Digit1", false}, "2": {"Digit2", false}, "3": {"Digit3", false},
	"4": {"Digit4", false}, "5": {"Digit5", false}, "6": {"Digit6", false},
	"7": {"Digit7", false}, "8": {"Digit8", false}, "9": {"Digit9", false},
	"0": {"Digit0", false},
	" ": {"Space", false},
	"-": {"Minus", false}, "=": {"Equal", false},
	"[": {"BracketLeft", false}, "]": {"BracketRight", false},
	"\\": {"Backslash", false}, ";": {"Semicolon", false},
	"'": {"Quote", false}, ",": {"Comma", false},
	".": {"Period", false}, "/": {"Slash", false},
	"`": {"Backquote", false},
	"!": {"Digit1", true}, "@": {"Digit2", true}, "#": {"Digit3", true},
	"$": {"Digit4", true}, "%": {"Digit5", true}, "^": {"Digit6", true},
	"&": {"Digit7", true}, "*": {"Digit8", true}, "(": {"Digit9", true},
	")": {"Digit0", true},
	"_": {"Minus", true}, "+": {"Equal", true},
	"{": {"BracketLeft", true}, "}": {"BracketRight", true},
	"|": {"Backslash", true}, ":": {"Semicolon", true},
	"\"": {"Quote", true}, "<": {"Comma", true},
	">": {"Period", true}, "?": {"Slash", true},
	"~": {"Backquote", true},
}

// buildPasteSequence 将文本转换为 paste API 可接受的 HID 键码序列。
func buildPasteSequence(text string) [][]string {
	var seq [][]string
	for _, r := range text {
		s := string(r)
		if info, ok := charToHIDKey[s]; ok {
			if info.shift {
				seq = append(seq, []string{"ShiftLeft", info.code})
			} else {
				seq = append(seq, []string{info.code})
			}
		}
		// 不支持的字符直接跳过
	}
	return seq
}
