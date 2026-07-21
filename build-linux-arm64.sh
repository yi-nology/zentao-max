#!/bin/bash
# 交叉编译 zentao-max 为 Linux arm64 可执行文件
# 详细说明见 build-linux.sh（Wails v3 + CGO 策略）

set -e
exec "$(dirname "$0")/build-linux.sh" arm64
