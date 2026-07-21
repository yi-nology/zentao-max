# Chandao Mini Makefile

# 前端相关命令
frontend-install:
	@echo "Installing frontend dependencies..."
	@cd frontend && npm install

frontend-dev:
	@echo "Running frontend in development mode..."
	@cd frontend && npm run dev

frontend-build:
	@echo "Building frontend..."
	@cd frontend && npm run build

# 后端相关命令
backend-install:
	@echo "Installing backend dependencies..."
	@cd backend && go mod tidy

backend-run:
	@echo "Running backend server..."
	@cd backend && go run cmd/server/main.go

backend-build:
	@echo "Building backend server..."
	@cd backend && go build -o server cmd/server/main.go

backend-app-run:
	@echo "Running backend app (with frontend static files)..."
	@cd backend && go run cmd/app/main.go

backend-app-build:
	@echo "Building backend app (with frontend static files)..."
	@cd backend && go build -o app cmd/app/main.go

# Wails 相关命令 (v3)
# v3 使用 Taskfile 作为主构建系统，这里通过 wails3 task 调用
WAILS_TAGS ?=
wails-build:
	@echo "Building Wails v3 application..."
	@wails3 task build

wails-run:
	@echo "Running Wails v3 application in dev mode..."
	@wails3 task dev

wails-generate:
	@echo "Regenerating Wails v3 bindings..."
	@wails3 generate bindings -d frontend/bindings

# 组合命令
install:
	@echo "Installing all dependencies..."
	@make frontend-install
	@go mod tidy

# 开发模式命令 - 不进行编译
run:
	@echo "Running application in development mode..."
	@make wails-run

# 版本构建命令 - 只有打版本时才进行编译
release:
	@echo "Building release version..."
	@make frontend-build
	@make wails-build
	@echo "Release build completed"

build:
	@echo "Building all components..."
	@make frontend-build
	@make wails-build

# 清理命令
clean:
	@echo "Cleaning build artifacts..."
	@rm -rf frontend/dist
	@rm -rf build

# 状态命令
status:
	@echo "Project status:"
	@echo "- Frontend: $(shell ls -la frontend/ | grep package.json | wc -l) package.json found"
	@echo "- Backend: $(shell ls -la backend/cmd/server/ | grep main.go | wc -l) main.go found"
	@echo "- Wails: $(shell ls -la | grep wails.json | wc -l) wails.json found"

# 帮助命令
help:
	@echo "Chandao Mini Makefile"
	@echo ""
	@echo "Frontend commands:"
	@echo "  make frontend-install   - Install frontend dependencies"
	@echo "  make frontend-dev       - Run frontend in development mode"
	@echo "  make frontend-build     - Build frontend"
	@echo ""
	@echo "Backend commands:"
	@echo "  make backend-install    - Install backend dependencies"
	@echo "  make backend-run        - Run backend server"
	@echo "  make backend-build      - Build backend server"
	@echo "  make backend-app-run    - Run backend app (with frontend static files)"
	@echo "  make backend-app-build  - Build backend app (with frontend static files)"
	@echo ""
	@echo "Wails commands:"
	@echo "  make wails-build        - Build Wails application"
	@echo "  make wails-run          - Run Wails application"
	@echo ""
	@echo "Combined commands:"
	@echo "  make install            - Install all dependencies"
	@echo "  make run                - Run application in development mode (no compilation)"
	@echo "  make build              - Build all components"
	@echo "  make release            - Build release version (only for versioning)"
	@echo ""
	@echo "Other commands:"
	@echo "  make clean              - Clean build artifacts"
	@echo "  make status             - Check project status"
	@echo "  make help               - Show this help"
