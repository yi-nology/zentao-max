#!/bin/bash
# 交叉编译 zentao-max 为 Linux 可执行文件
#
# === Wails v3 迁移后重要变化 ===
# v3 必须启用 CGO（webview 集成），不再支持 CGO_ENABLED=0 的纯 Go 编译。
# 这意味着无法从 macOS/Windows 直接交叉编译 Linux 桌面包（缺 GTK/WebKit cross-toolchain）。
#
# 推荐方案（按优先级）：
#   1. CI 构建：推送到 master 或打 tag，GitHub Actions 在 Linux runner 上构建
#   2. Docker 交叉编译：wails3 task setup:docker && wails3 task linux:build
#   3. 本地 Linux 环境：直接运行本脚本（需先装系统依赖）
#
# 本脚本现在调用 wails3 task，需要 wails3 CLI 已安装：
#   go install github.com/wailsapp/wails/v3/cmd/wails3@latest

set -e

ARCH="${1:-amd64}"  # 默认 amd64，可传 arm64

echo "=== 构建 zentao-max Linux/$ARCH 版本 ==="

if ! command -v wails3 >/dev/null 2>&1; then
    echo "错误: wails3 CLI 未安装"
    echo "  go install github.com/wailsapp/wails/v3/cmd/wails3@latest"
    exit 1
fi

if [[ "$(uname -s)" != "Linux" ]]; then
    echo "警告: 当前不是 Linux 环境"
    echo "  v3 + CGO 桌面应用无法跨平台编译"
    echo "  选项 A: 在 Linux 机器/VM 中运行本脚本"
    echo "  选项 B: 用 Docker 交叉编译:"
    echo "          wails3 task setup:docker"
    echo "          wails3 task linux:build"
    echo "  选项 C: 仅构建 server 模式（无 GUI，纯 HTTP）:"
    echo "          cd backend && CGO_ENABLED=0 GOOS=linux GOARCH=$ARCH go build -o ../build/bin/zentao-max-server ./cmd/server"
    echo ""
    read -p "继续尝试本机构建 server 模式? (y/N) " confirm
    if [[ "$confirm" != "y" ]]; then
        exit 1
    fi
    cd backend && CGO_ENABLED=0 GOOS=linux GOARCH=$ARCH go build -o ../build/bin/zentao-max-server ./cmd/server
    echo "✓ server 二进制: build/bin/zentao-max-server"
    exit 0
fi

# Linux 本地环境，装系统依赖（首次需要）
echo "检查系统依赖..."
if ! pkg-config --exists gtk+-3.0 webkit2gtk-4.1 2>/dev/null; then
    echo "安装 GTK/WebKit 依赖..."
    sudo apt-get update
    sudo apt-get install -y \
        build-essential pkg-config \
        libgtk-3-dev libwebkit2gtk-4.1-dev libglib2.0-dev \
        libayatana-appindicator3-dev \
        patchelf file fakeroot desktop-file-utils
fi

# 调用 wails3 task 构建（自动处理 frontend + Go + 打包）
echo "调用 wails3 task linux:build..."
GOARCH=$ARCH wails3 task linux:build

echo ""
echo "✓ 构建完成"
echo "  产物位置: build/bin/"
ls -lah build/bin/ 2>/dev/null || true
