.PHONY: build start stop restart start-backend stop-backend start-frontend stop-frontend logs help

# 项目配置
BINARY_NAME=c2c_monitor
FRONTEND_DIR=frontend
FRONTEND_PORT=8080
BACKEND_PORT=8001

# 默认目标
help:
	@echo "C2C Monitor 服务管理"
	@echo ""
	@echo "使用方法:"
	@echo "  make build           - 编译后端"
	@echo "  make start           - 启动所有服务 (后端+前端)"
	@echo "  make stop            - 关闭所有服务"
	@echo "  make restart         - 重启所有服务"
	@echo ""
	@echo "  make start-backend   - 仅启动后端"
	@echo "  make stop-backend    - 仅关闭后端"
	@echo "  make start-frontend  - 仅启动前端"
	@echo "  make stop-frontend   - 仅关闭前端"
	@echo ""
	@echo "  make logs            - 查看后端日志"
	@echo "  make status          - 查看服务状态"

# 编译后端
build:
	@echo "编译后端..."
	go build -o $(BINARY_NAME) ./cmd/monitor
	@echo "编译完成"

# 启动后端
start-backend: build
	@echo "启动后端服务..."
	@pkill -f "./$(BINARY_NAME)" 2>/dev/null || true
	@sleep 1
	@nohup ./$(BINARY_NAME) > logs/backend.log 2>&1 &
	@sleep 2
	@if curl -s http://localhost:$(BACKEND_PORT)/api/config > /dev/null; then \
		echo "后端启动成功 - http://localhost:$(BACKEND_PORT)"; \
	else \
		echo "后端启动失败，请检查日志: logs/backend.log"; \
	fi

# 关闭后端
stop-backend:
	@echo "关闭后端服务..."
	@pkill -f "./$(BINARY_NAME)" 2>/dev/null || true
	@echo "后端已关闭"

# 启动前端
start-frontend:
	@echo "启动前端服务..."
	@pkill -f "python3 frontend/dev_server.py" 2>/dev/null || true
	@sleep 1
	@nohup python3 frontend/dev_server.py $(FRONTEND_PORT) $(FRONTEND_DIR) > logs/frontend.log 2>&1 &
	@sleep 1
	@echo "前端启动成功 - http://localhost:$(FRONTEND_PORT) (No-Cache Mode)"

# 关闭前端
stop-frontend:
	@echo "关闭前端服务..."
	@pkill -f "python3 frontend/dev_server.py" 2>/dev/null || true
	@echo "前端已关闭"

# 启动所有服务
start:
	@mkdir -p logs
	@$(MAKE) start-backend
	@$(MAKE) start-frontend
	@echo ""
	@echo "所有服务已启动"
	@echo "  前端: http://localhost:$(FRONTEND_PORT)"
	@echo "  后端: http://localhost:$(BACKEND_PORT)"

# 关闭所有服务
stop:
	@$(MAKE) stop-backend
	@$(MAKE) stop-frontend
	@echo "所有服务已关闭"

# 重启所有服务
restart:
	@$(MAKE) stop
	@sleep 1
	@$(MAKE) start

# 查看日志
logs:
	@tail -f logs/backend.log

# 查看服务状态
status:
	@echo "服务状态:"
	@echo -n "  后端: "
	@if pgrep -f "./$(BINARY_NAME)" > /dev/null; then echo "运行中"; else echo "已停止"; fi
	@echo -n "  前端: "
	@if pgrep -f "http.server $(FRONTEND_PORT)" > /dev/null; then echo "运行中"; else echo "已停止"; fi
