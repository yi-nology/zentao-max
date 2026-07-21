#!/bin/bash
# 交叉编译 zentao-max 为 Windows x64 可执行文件
#
# === Wails v3 迁移后重要变化 ===
# v3 必须启用 CGO（webview2 集成），不再支持 CGO_ENABLED=0 的纯 Go 编译。
# macOS/Linux 上无法直接交叉编译 Windows 桌面包（缺 mingw + webview2 cross-toolchain）。
#
# 推荐方案：
#   1. CI 构建：GitHub Actions 在 Windows runner 上构建（最简单）
#   2. Windows 本地环境：在 Windows 上 wsl/git-bash 运行本脚本
#   3. 仅构建 server 模式：cd backend && CGO_ENABLED=0 GOOS=windows go build ./cmd/server

set -e

echo "=== 构建 zentao-max Windows x64 版本 ==="

if ! command -v wails3 >/dev/null 2>&1; then
    echo "错误: wails3 CLI 未安装"
    echo "  go install github.com/wailsapp/wails/v3/cmd/wails3@latest"
    exit 1
fi

if [[ "$(uname -s)" == "MINGW"* || "$(uname -s)" == "MSYS"* || "$(uname -s)" == "CYGWIN"* ]]; then
    # Windows 本地环境
    echo "检测到 Windows 环境，调用 wails3 task windows:build..."
    wails3 task windows:build
    echo "✓ 构建完成"
    ls -lah build/bin/ 2>/dev/null || true
    exit 0
fi

# 非 Windows 环境
echo "警告: 当前不是 Windows 环境"
echo "  v3 + CGO 桌面应用无法跨平台编译为 Windows .exe"
echo "  选项 A: 推送到 GitHub，让 CI 在 Windows runner 上构建"
echo "  选项 B: 用 Docker: wails3 task setup:docker && wails3 task windows:build"
echo "  选项 C: 仅构建 server 模式（无 GUI）:"
echo "          cd backend && CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o ../build/bin/zentao-max-server.exe ./cmd/server"
echo ""
read -p "继续尝试本机构建 server 模式? (y/N) " confirm
if [[ "$confirm" != "y" ]]; then
    exit 1
fi
cd backend && CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o ../build/bin/zentao-max-server.exe ./cmd/server
echo "✓ server 二进制: build/bin/zentao-max-server.exe"
