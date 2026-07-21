#!/bin/bash

# 复制前端静态资源到后端目录
set -e

if [ ! -d "frontend/dist" ]; then
    echo "错误: frontend/dist 目录不存在，请先运行 npm run build"
    exit 1
fi

# 创建后端静态资源目录
mkdir -p backend/cmd/app/static

# 复制前端构建后的静态资源
cp -r frontend/dist/* backend/cmd/app/static/

echo "Static files copied to backend/cmd/app/static/"
