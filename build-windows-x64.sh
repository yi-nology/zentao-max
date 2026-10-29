#!/bin/bash
# 交叉编译 zentao-max 为 Windows amd64 可执行文件（纯 Web 单二进制：HTTP server + 嵌入式前端）
#
# 纯 Go 技术栈（hertz + modernc.org/sqlite），CGO_ENABLED=0 即可从 macOS/Linux 直接
# 交叉编译 .exe，与 CI（.github/workflows/build.yml）同流程。

set -e

echo "=== 构建 zentao-max windows/amd64 ==="

# 1. 前端构建产物（嵌入后端）
if [ ! -d "frontend/dist" ]; then
    echo "-- frontend/dist 不存在，执行前端构建 --"
    (cd frontend && npm install && npm run build)
fi

# 2. 静态资源就位（先清空防旧 chunk 残留）
rm -rf backend/static
./scripts/copy-static.sh

# 3. 交叉编译
echo "-- CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build --"
cd backend
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "-s -w" -o ../zentao-max-windows-amd64.exe .
cd ..

echo "✓ 产物: zentao-max-windows-amd64.exe"
echo "  运行: 在 Windows 上 set PORT=8080 && zentao-max-windows-amd64.exe"
