# blikvm-mcp-server

一个 MCP (Model Context Protocol) 服务器，将 BliKVM 的截图和键鼠控制能力暴露为 MCP 工具，让 AI agent 可以通过 MCP 协议远程控制被控 PC。

## 功能

将 BliKVM Web Server 的 REST API 封装为标准 MCP 工具，供任何支持 MCP 的客户端（如 Trae IDE、Claude Desktop 等）调用。

### 提供的 MCP 工具

| 工具名 | 功能 |
|--------|------|
| `blikvm_screenshot` | 截取远程屏幕，返回 JPEG 图片 |
| `blikvm_mouse_move` | 移动鼠标到归一化坐标 (x, y ∈ [0,1]) |
| `blikvm_mouse_click` | 单击鼠标按钮（left/right/middle，可选先移动） |
| `blikvm_mouse_double_click` | 双击鼠标 |
| `blikvm_mouse_drag` | 从一点拖拽到另一点 |
| `blikvm_mouse_scroll` | 滚动鼠标滚轮 |
| `blikvm_key_tap` | 按下并释放单个键盘键 |
| `blikvm_key_hotkey` | 按下组合键（如 Ctrl+C、Ctrl+Shift+V） |
| `blikvm_type_text` | 通过键盘输入文本字符串 |
| `blikvm_atx_power_control` | 控制被控主机电源（power/force_off/reset_hard） |
| `blikvm_atx_get_status` | 获取 ATX 电源和 LED 状态 |
| `blikvm_bios_start` | 开始 BIOS 进入模式（周期性发送按键） |
| `blikvm_bios_stop` | 停止 BIOS 进入模式 |
| `blikvm_bios_get_status` | 获取 BIOS 进入模式状态 |

## 编译

### 本机编译

```bash
cd blikvm-mcp-server
go build -o blikvm-mcp-server
```

### 交叉编译到 macOS

```bash
# macOS ARM64 (Apple Silicon)
GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build -o blikvm-mcp-server-darwin-arm64 ./...

# macOS AMD64 (Intel)
GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build -o blikvm-mcp-server-darwin-amd64 ./...
```

### 交叉编译到 Windows

```bash
# Windows AMD64
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -o blikvm-mcp-server-windows-amd64.exe ./...
```

### 交叉编译到 rk3566 (Buildroot 工具链)

BliKVM 的目标设备基于 Rockchip RK3566 (ARM64)，运行 Buildroot 构建的 Linux 系统。本仓库为纯 Go 实现（无 cgo 依赖），因此交叉编译非常简单，无需 C 工具链。

#### 方式一：直接使用 Go 交叉编译（推荐）

由于本项目不依赖 cgo，可直接用 Go 的内置交叉编译能力：

```bash
cd blikvm-mcp-server
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o blikvm-mcp-server ./...
```

产物 `blikvm-mcp-server` 即为可在 rk3566 设备上运行的 ARM64 二进制。

#### 方式二：使用 Buildroot 工具链（如需 cgo）

如果你有 Buildroot SDK（例如从 `/path/to/buildroot` 构建得到），可以使用其生成的工具链：

```bash
# 设置 Buildroot 工具链路径
export PATH="/path/to/buildroot/output/rockchip_rk3566/host/bin:$PATH"
export CC=/path/to/buildroot/output/rockchip_rk3566/host/bin/aarch64-buildroot-linux-gnu-gcc
export CXX=/path/to/buildroot/output/rockchip_rk3566/host/bin/aarch64-buildroot-linux-gnu-g++

# 交叉编译
cd blikvm-mcp-server
CGO_ENABLED=1 GOOS=linux GOARCH=arm64 CC=$CC go build -o blikvm-mcp-server ./...
```

#### 实际示例

```bash
export PATH="/home/blikvm/.local/go/bin:$PATH"
export CC=/home/blikvm/Desktop/buildroot/output/rockchip_rk3566/host/bin/aarch64-buildroot-linux-gnu-gcc
export CXX=/home/blikvm/Desktop/buildroot/output/rockchip_rk3566/host/bin/aarch64-buildroot-linux-gnu-g++

cd /home/blikvm/Downloads/blikvm-release/dist/work/sources/blikvm-mcp-server
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o blikvm-mcp-server ./...
```

#### 验证产物架构

```bash
file blikvm-mcp-server
# 预期输出类似：ELF 64-bit LSB executable, ARM aarch64, version 1 (SYSV), statically linked, ...
```

### 部署到设备

将编译好的二进制通过 scp 或其他方式传到 BliKVM 设备：

```bash
scp blikvm-mcp-server root@<blikvm-ip>:/usr/local/bin/
ssh root@<blikvm-ip> "chmod +x /usr/local/bin/blikvm-mcp-server"
```

## 使用方式

### 命令行参数

```bash
./blikvm-mcp-server -url http://<blikvm-ip> -username <user> -password <pass>
```

| 参数 | 说明 |
|------|------|
| `-url` | BliKVM Web Server 地址，例如 `http://192.168.1.100` |
| `-username` | 登录用户名 |
| `-password` | 登录密码 |

### 环境变量

也可通过环境变量配置（优先级低于命令行参数）：

```bash
export BLIKVM_URL=http://192.168.1.100
export BLIKVM_USERNAME=admin
export BLIKVM_PASSWORD=yourpass
./blikvm-mcp-server
```

### 在 Trae IDE 中配置

在 Trae 的设置 → MCP Servers 中添加：
方案 1：在本地运行 MCP server，连接远程 blikvm
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

{
  "mcpServers": {
    "blikvm": {
      "command": "/home/blikvm/Downloads/blikvm-release/dist/work/sources/blikvm-mcp-server/blikvm-mcp-server",
      "args": [
        "-url", "https://10.0.0.10",
        "-username", "admin",
        "-password", "admin"
      ]
    }
  }
}
```
```
方案 2：SSH 远程启动
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

### 在 Claude Desktop 中配置

编辑 `claude_desktop_config.json`：

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

配置完成后，AI agent 即可直接调用 `blikvm_screenshot`、`blikvm_mouse_click` 等工具来观察并控制远程 PC。

## 工作原理

1. 启动时使用用户名/密码调用 `/api/v1/auth/login` 获取 Bearer token
2. 后续所有请求都带上 `Authorization: Bearer <token>`
3. 遇到 401 时自动重新登录并重试一次
4. 通过 stdio 与 MCP 客户端通信（标准 MCP 传输方式）

### 调用的 BliKVM API

| MCP 工具 | BliKVM API |
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

## 坐标系统

鼠标坐标使用 **归一化绝对坐标**，范围 `[0.0, 1.0]`：
- `(0, 0)` = 屏幕左上角
- `(1, 1)` = 屏幕右下角
- `(0.5, 0.5)` = 屏幕中心

AI agent 应先调用 `blikvm_screenshot` 查看屏幕，再根据截图估算目标位置的归一化坐标。

## 项目结构

```
blikvm-mcp-server/
├── main.go       # 入口：解析参数，启动 stdio MCP server
├── client.go     # BliKVM REST API 客户端（登录/截图/键鼠/ATX/BIOS）
├── tools.go      # 14 个 MCP 工具定义
├── go.mod
└── README.md
```

## 许可证

同 BliKVM 主项目。
