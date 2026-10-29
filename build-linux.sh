#!/bin/bash
# 交叉编译 zentao-max 为 Linux 可执行文件（纯 Web 单二进制：HTTP server + 嵌入式前端）
#
# 纯 Go 技术栈（hertz + modernc.org/sqlite），CGO_ENABLED=0 即可从任意平台交叉编译，
# 与 CI（.github/workflows/build.yml「Build zentao-max」）同流程。
#
# 用法: ./build-linux.sh [amd64|arm64]   （默认 amd64）

set -e

ARCH="${1:-amd64}"

echo "=== 构建 zentao-max linux/$ARCH ==="

# 1. 前端构建产物（嵌入后端）
if [ ! -d "frontend/dist" ]; then
    echo "-- frontend/dist 不存在，执行前端构建 --"
    (cd frontend && npm install && npm run build)
fi

# 2. 静态资源就位（脚本为普通 cp 非 mirror，先清空防旧 chunk 残留）
rm -rf backend/static
./scripts/copy-static.sh

# 3. 交叉编译
echo "-- CGO_ENABLED=0 GOOS=linux GOARCH=$ARCH go build --"
cd backend
CGO_ENABLED=0 GOOS=linux GOARCH="$ARCH" go build -trimpath -ldflags "-s -w" -o "../zentao-max-linux-$ARCH" .
cd ..

echo "✓ 产物: zentao-max-linux-$ARCH"
echo "  运行: PORT=8080 ./zentao-max-linux-$ARCH"
