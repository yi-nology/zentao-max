#!/bin/bash
# 交叉编译 zentao-max 为 Linux arm64 可执行文件（见 build-linux.sh 说明）

set -e
exec "$(dirname "$0")/build-linux.sh" arm64
