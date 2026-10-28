# Makefile for zentao-max backend

.PHONY: all build build-mcp run clean test test-coverage test-coverage-html lint fmt help

# 构建目标
all: build

# 构建项目
build:
	go build -o zentao-max .

# 构建 MCP stdio 独立入口（供 AI 工具作为子进程启动）
build-mcp:
	go build -o zentao-max-mcp ./cmd/mcp

# 运行项目
run: build
	./server

# 清理构建产物
clean:
	rm -f server zentao-max-mcp
	rm -f coverage.out coverage.html

# 运行所有测试
test:
	go test -v ./...

# 运行测试并生成覆盖率报告
test-coverage:
	go test -v -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

# 生成HTML格式的覆盖率报告
test-coverage-html:
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "覆盖率报告已生成: coverage.html"

# 运行代码检查
lint:
	@which golangci-lint > /dev/null || (echo "请先安装 golangci-lint: https://golangci-lint.run/usage/install/" && exit 1)
	golangci-lint run ./...

# 运行代码检查（自动修复）
lint-fix:
	@which golangci-lint > /dev/null || (echo "请先安装 golangci-lint: https://golangci-lint.run/usage/install/" && exit 1)
	golangci-lint run --fix ./...

# 格式化代码
fmt:
	go fmt ./...

# 运行所有检查（格式化 + lint + 测试）
check: fmt lint test

# 安装开发工具
install-tools:
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
	@echo "开发工具安装完成"

# 显示帮助信息
help:
	@echo "可用命令:"
	@echo "  make build              - 构建项目（HTTP server）"
	@echo "  make build-mcp          - 构建 MCP stdio 独立入口（zentao-max-mcp）"
	@echo "  make run                - 运行项目"
	@echo "  make test               - 运行所有测试"
	@echo "  make test-coverage      - 运行测试并生成覆盖率报告"
	@echo "  make test-coverage-html - 生成HTML格式的覆盖率报告"
	@echo "  make lint               - 运行代码检查"
	@echo "  make lint-fix           - 运行代码检查并自动修复"
	@echo "  make fmt                - 格式化代码"
	@echo "  make check              - 运行所有检查（格式化 + lint + 测试）"
	@echo "  make install-tools      - 安装开发工具"
