# zentao-max Makefile

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
	@echo "Running backend (static: embedded or -static <dir>)..."
	@cd backend && go run .

backend-build:
	@echo "Building backend single binary..."
	@cd backend && go build -o zentao-max .





# 开发模式（前端 vite dev 代理 /api，后端 -static 磁盘或 embedded）
run:
	@echo "Running backend in development mode..."
	@cd backend && go run .

# 发布构建：前端构建 + 静态资源就位 + 单二进制
release:
	@echo "Building release version..."
	@make frontend-build
	@rm -rf backend/static
	@./scripts/copy-static.sh
	@cd backend && go build -trimpath -ldflags "-s -w -X github.com/yi-nology/zentao-max/backend/internal/version.Version=$$(date +%Y.%m.%d)" -o zentao-max .
	@echo "Release build completed"

build:
	@echo "Building all components..."
	@make frontend-build
	@rm -rf backend/static
	@./scripts/copy-static.sh
	@cd backend && go build -o zentao-max .

# 清理命令
clean:
	@echo "Cleaning build artifacts..."
	@rm -rf frontend/dist backend/static

# 状态命令
status:
	@echo "Project status:"
	@echo "- Frontend: $(shell ls -la frontend/ | grep package.json | wc -l) package.json found"
	@echo "- Backend entry: backend/main.go"

# 帮助命令
help:
	@echo "zentao-max Makefile"
	@echo ""
	@echo "Frontend commands:"
	@echo "  make frontend-install   - Install frontend dependencies"
	@echo "  make frontend-dev       - Run frontend in development mode"
	@echo "  make frontend-build     - Build frontend"
	@echo ""
	@echo "Backend commands:"
	@echo "  make backend-install    - Install backend dependencies"
	@echo "  make backend-run        - Run backend (embedded static by default)"
	@echo "  make backend-build      - Build backend single binary"
	@echo ""
	@echo "Combined commands:"
	@echo "  make install            - Install all dependencies"
	@echo "  make run                - Run backend in development mode"
	@echo "  make build              - Build all components"