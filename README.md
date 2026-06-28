# blikvm-mcp-server

[中文文档](README.zh.md)

An MCP (Model Context Protocol) server that exposes BliKVM's screenshot and HID (keyboard/mouse) control capabilities as MCP tools, enabling AI agents to remotely observe and control the target PC through the MCP protocol.

## Features

Wraps BliKVM Web Server REST APIs into standard MCP tools for any MCP-compatible client (Trae IDE, Claude Desktop, etc.).

### Provided MCP Tools

| Tool | Function |
|------|----------|
| `blikvm_screenshot` | Capture remote screen and return JPEG image |
| `blikvm_mouse_move` | Move mouse to normalized coordinates (x, y ∈ [0,1]) |
| `blikvm_mouse_click` | Click mouse button (left/right/middle, optional move first) |
| `blikvm_mouse_double_click` | Double-click mouse |
| `blikvm_mouse_drag` | Drag from one point to another |
| `blikvm_mouse_scroll` | Scroll mouse wheel |
| `blikvm_key_tap` | Press and release a single keyboard key |
| `blikvm_key_hotkey` | Press keyboard shortcut (e.g. Ctrl+C, Ctrl+Shift+V) |
| `blikvm_type_text` | Type text string via keyboard |
| `blikvm_atx_power_control` | Control remote host power (power/force_off/reset_hard) |
| `blikvm_atx_get_status` | Get ATX power and LED status |
| `blikvm_bios_start` | Start BIOS access mode (periodic key sending) |
| `blikvm_bios_stop` | Stop BIOS access mode |
| `blikvm_bios_get_status` | Get BIOS access mode status |

## Build

### Native Build

```bash
cd blikvm-mcp-server
go build -o blikvm-mcp-server
```

### Cross-Compile for macOS

```bash
# macOS ARM64 (Apple Silicon)
GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build -o blikvm-mcp-server-darwin-arm64 ./...

# macOS AMD64 (Intel)
GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build -o blikvm-mcp-server-darwin-amd64 ./...
```

### Cross-Compile for Windows

```bash
# Windows AMD64
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -o blikvm-mcp-server-windows-amd64.exe ./...
```

### Cross-Compile for rk3566 (Buildroot Toolchain)

BliKVM targets Rockchip RK3566 (ARM64) devices running Buildroot Linux. This project is pure Go (no cgo), so cross-compilation is straightforward without a C toolchain.

#### Method 1: Direct Go Cross-Compilation (Recommended)

Since this project has no cgo dependency, use Go's built-in cross-compilation:

```bash
cd blikvm-mcp-server
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o blikvm-mcp-server ./...
```

The output `blikvm-mcp-server` is an ARM64 binary ready to run on rk3566 devices.

#### Method 2: Using Buildroot Toolchain (if cgo needed)

If you have a Buildroot SDK (e.g. from `/path/to/buildroot`):

```bash
# Set Buildroot toolchain path
export PATH="/path/to/buildroot/output/rockchip_rk3566/host/bin:$PATH"
export CC=/path/to/buildroot/output/rockchip_rk3566/host/bin/aarch64-buildroot-linux-gnu-gcc
export CXX=/path/to/buildroot/output/rockchip_rk3566/host/bin/aarch64-buildroot-linux-gnu-g++

# Cross-compile
cd blikvm-mcp-server
CGO_ENABLED=1 GOOS=linux GOARCH=arm64 CC=$CC go build -o blikvm-mcp-server ./...
```

#### Practical Example

```bash
export PATH="/home/blikvm/.local/go/bin:$PATH"
export CC=/home/blikvm/Desktop/buildroot/output/rockchip_rk3566/host/bin/aarch64-buildroot-linux-gnu-gcc
export CXX=/home/blikvm/Desktop/buildroot/output/rockchip_rk3566/host/bin/aarch64-buildroot-linux-gnu-g++

cd /home/blikvm/Downloads/blikvm-release/dist/work/sources/blikvm-mcp-server
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o blikvm-mcp-server ./...
```

#### Verify Output Architecture

```bash
file blikvm-mcp-server
# Expected: ELF 64-bit LSB executable, ARM aarch64, version 1 (SYSV), statically linked, ...
```

### Deploy to Device

Copy the binary to the BliKVM device via scp:

```bash
scp blikvm-mcp-server root@<blikvm-ip>:/usr/local/bin/
ssh root@<blikvm-ip> "chmod +x /usr/local/bin/blikvm-mcp-server"
```

## Usage

### Command Line Arguments

```bash
./blikvm-mcp-server -url http://<blikvm-ip> -username <user> -password <pass>
```

| Argument | Description |
|----------|-------------|
| `-url` | BliKVM Web Server address, e.g. `http://192.168.1.100` |
| `-username` | Login username |
| `-password` | Login password |

### Environment Variables

Alternatively, use environment variables (lower priority than CLI args):

```bash
export BLIKVM_URL=http://192.168.1.100
export BLIKVM_USERNAME=admin
export BLIKVM_PASSWORD=yourpass
./blikvm-mcp-server
```

### Configure in Trae IDE

Add to Trae Settings → MCP Servers:

#### Option 1: Run MCP server locally, connect to remote blikvm

```json
{
  "mcpServers": {
    "blikvm": {
      "command": "/usr/local/bin/blikvm-mcp-server",
      "args": [
        "-url", "http://192.168.1.100",
        "-username", "admin",
        "-password", "yourpass"
      ]
    }
  }
}
```

#### Option 2: SSH remote launch

```json
{
  "mcpServers": {
    "blikvm": {
      "command": "ssh",
      "args": [
        "root@192.168.1.100",
        "/usr/local/bin/blikvm-mcp-server",
        "-url", "http://localhost",
        "-username", "admin",
        "-password", "yourpass"
      ]
    }
  }
}
```

### Configure in Claude Desktop

Edit `claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "blikvm": {
      "command": "/usr/local/bin/blikvm-mcp-server",
      "args": [
        "-url", "http://192.168.1.100",
        "-username", "admin",
        "-password", "yourpass"
      ]
    }
  }
}
```

After configuration, the AI agent can directly call `blikvm_screenshot`, `blikvm_mouse_click`, etc. to observe and control the remote PC.

## How It Works

1. On startup, calls `/api/v1/auth/login` with username/password to obtain a Bearer token
2. All subsequent requests include `Authorization: Bearer <token>`
3. On 401 responses, automatically re-authenticates and retries once
4. Communicates with the MCP client via stdio (standard MCP transport)

### BliKVM APIs Used

| MCP Tool | BliKVM API |
|----------|-----------|
| `blikvm_screenshot` | `GET /api/v1/video/snapshot` |
| `blikvm_mouse_move` | `POST /api/v1/hid/events` (type=mouseMove) |
| `blikvm_mouse_click` | `POST /api/v1/hid/events` (type=mouseButton) |
| `blikvm_mouse_drag` | `POST /api/v1/hid/events` (type=mouseMove + mouseButton) |
| `blikvm_mouse_scroll` | `POST /api/v1/hid/events` (type=mouseWheel) |
| `blikvm_key_tap` | `POST /api/v1/hid/events` (type=keyboard) |
| `blikvm_key_hotkey` | `POST /api/v1/hid/paste` |
| `blikvm_type_text` | `POST /api/v1/hid/paste` |
| `blikvm_atx_power_control` | `POST /api/v1/atx/power` |
| `blikvm_atx_get_status` | `GET /api/v1/atx` |
| `blikvm_bios_start` | `POST /api/v1/atx/bios/start` |
| `blikvm_bios_stop` | `POST /api/v1/atx/bios/stop` |
| `blikvm_bios_get_status` | `GET /api/v1/atx/bios/status` |

## Coordinate System

Mouse coordinates use **normalized absolute coordinates**, range `[0.0, 1.0]`:
- `(0, 0)` = top-left corner
- `(1, 1)` = bottom-right corner
- `(0.5, 0.5)` = screen center

The AI agent should first call `blikvm_screenshot` to see the screen, then estimate the normalized coordinates of the target position.

## Project Structure

```
blikvm-mcp-server/
├── main.go       # Entry: parse args, start stdio MCP server
├── client.go     # BliKVM REST API client (login/screenshot/HID/ATX/BIOS)
├── tools.go      # 14 MCP tool definitions
├── go.mod
└── README.md
```

## License

Same as the BliKVM main project.
