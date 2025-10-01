# 项目配置
SCRIPTS_DIR := ./scripts
DEPLOYMENTS_DIR := ./deployments/prometheus

# 颜色定义
RED := \033[0;31m
GREEN := \033[0;32m
BLUE := \033[0;34m
YELLOW := \033[1;33m
NC := \033[0m

# 依赖检查
.PHONY: proto_deps
proto_deps:
	@echo "$(BLUE)Checking dependencies...$(NC)"
	@command -v protoc >/dev/null 2>&1 || { echo "$(RED)protoc is required but not installed$(NC)"; exit 1; }
	@command -v protoc-gen-go >/dev/null 2>&1 || go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	@command -v protoc-gen-go-grpc >/dev/null 2>&1 || go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	@echo "$(GREEN)Dependencies check passed$(NC)"

# Protobuf 生成命令 - 支持灵活目录映射
.PHONY: proto
proto: proto_deps
	@echo "$(BLUE)Generating protobuf files with flexible directory mapping...$(NC)"
	@$(SCRIPTS_DIR)/gen_proto.sh

.PHONY: proto-force
proto-force: proto_deps
	@echo "$(BLUE)Force regenerating all proto files...$(NC)"
	@$(SCRIPTS_DIR)/gen_proto.sh --force

.PHONY: proto-clean
proto-clean:
	@echo "$(BLUE)Cleaning generated proto files from all directories...$(NC)"
	@$(SCRIPTS_DIR)/gen_proto.sh --clean

# 显示当前目录映射
.PHONY: proto-mapping
proto-mapping:
	@$(SCRIPTS_DIR)/gen_proto.sh --mapping

# 验证所有目录的生成结果
.PHONY: verify-proto
verify-proto: proto_deps
	@echo "$(BLUE)Validating generated protobuf files...$(NC)"
	@$(SCRIPTS_DIR)/gen_proto.sh --force
	@go build ./net/... ./game/... ./user/... ./battle/... ./state/... ./proto/... 2>/dev/null || true

# 按模块分别生成
.PHONY: proto-net
proto-net: proto_deps
	@echo "$(BLUE)Generating network-related protobuf files...$(NC)"
	@cd $(PROTO_DIR) && for f in *net*.proto; do \
		if [ -f "$$f" ]; then \
			$(SCRIPTS_DIR)/gen_proto.sh --force | grep -E "(Generating.*→.*net/|Success.*net)"; \
		fi; \
	done

.PHONY: proto-game
proto-game: proto_deps
	@echo "$(BLUE)Generating game logic protobuf files...$(NC)"
	@cd $(PROTO_DIR) && for f in *game*.proto; do \
		if [ -f "$$f" ]; then \
			$(SCRIPTS_DIR)/gen_proto.sh --force | grep -E "(Generating.*→.*game/|Success.*game)"; \
		fi; \
	done

# 开发模式：监控文件变化自动重新生成
.PHONY: proto-watch
proto-watch: proto_deps
	@echo "$(YELLOW)Watching proto files for changes...$(NC)"
	@command -v inotifywait >/dev/null 2>&1 || { echo "$(RED)inotifywait is required for watch mode$(NC)"; exit 1; }
	@while true; do \
		inotifywait -e modify,create,delete -r $(PROTO_DIR) --exclude '\.swp$$'; \
		echo "$(GREEN)Proto files changed, regenerating...$(NC)"; \
		$(SCRIPTS_DIR)/gen_proto.sh; \
	done

# ==========================================
# Prometheus 部署相关命令
# ==========================================

# 构建应用镜像
.PHONY: build-app-image
build-app-image: check-docker
	@echo "$(BLUE)构建应用镜像...$(NC)"
	@docker build -t asura-app:latest -f $(DEPLOYMENTS_DIR)/Dockerfile.app $(DEPLOYMENTS_DIR)/..
	@echo "$(GREEN)应用镜像构建完成$(NC)"

# 检查 Docker 环境
.PHONY: check-docker
check-docker:
	@echo "$(BLUE)检查 Docker 环境...$(NC)"
	@command -v docker >/dev/null 2>&1 || { echo "$(RED)Docker 未安装$(NC)"; exit 1; }
	@command -v docker-compose >/dev/null 2>&1 || { echo "$(RED)Docker Compose 未安装$(NC)"; exit 1; }
	@echo "$(GREEN)Docker 环境检查通过$(NC)"

# 部署 Prometheus
.PHONY: deploy-prometheus
deploy-prometheus: check-docker
	@echo "$(GREEN)开始部署 Prometheus...$(NC)"
	@cd $(DEPLOYMENTS_DIR) && docker-compose up -d
	@echo "$(YELLOW)等待服务启动...$(NC)"
	@sleep 10
	@echo "$(GREEN)Prometheus 部署完成！$(NC)"
	@echo "$(YELLOW)访问地址:$(NC)"
	@echo "  Prometheus: http://localhost:9090"
	@echo "  Grafana: http://localhost:3000 (admin/admin)"

# 停止 Prometheus
.PHONY: stop-prometheus
stop-prometheus: check-docker
	@echo "$(YELLOW)停止 Prometheus 服务...$(NC)"
	@cd $(DEPLOYMENTS_DIR) && docker-compose down
	@echo "$(GREEN)Prometheus 服务已停止$(NC)"

# 清理 Prometheus 资源
.PHONY: cleanup-prometheus
cleanup-prometheus: check-docker
	@echo "$(YELLOW)清理 Prometheus 资源...$(NC)"
	@cd $(DEPLOYMENTS_DIR) && docker-compose down -v
	@echo "$(GREEN)Prometheus 资源已清理$(NC)"

# 查看 Prometheus 状态
.PHONY: status-prometheus
status-prometheus: check-docker
	@echo "$(BLUE)Prometheus 服务状态:$(NC)"
	@cd $(DEPLOYMENTS_DIR) && docker-compose ps

# 查看 Prometheus 日志
.PHONY: logs-prometheus
logs-prometheus: check-docker
	@echo "$(BLUE)Prometheus 服务日志:$(NC)"
	@cd $(DEPLOYMENTS_DIR) && docker-compose logs -f

# 重启 Prometheus
.PHONY: restart-prometheus
restart-prometheus: check-docker
	@echo "$(YELLOW)重启 Prometheus 服务...$(NC)"
	@cd $(DEPLOYMENTS_DIR) && docker-compose restart
	@echo "$(GREEN)Prometheus 服务已重启$(NC)"

# 快速部署（完整流程）
.PHONY: prometheus
prometheus: deploy-prometheus
	@echo "$(GREEN)Prometheus 快速部署完成！$(NC)"

# 开发环境部署（带热重载）
.PHONY: prometheus-dev
prometheus-dev: check-docker
	@echo "$(GREEN)启动开发环境 Prometheus...$(NC)"
	@cd $(DEPLOYMENTS_DIR) && docker-compose up

# 生产环境部署
.PHONY: prometheus-prod
prometheus-prod: check-docker
	@echo "$(GREEN)开始生产环境部署...$(NC)"
	@cd $(DEPLOYMENTS_DIR) && docker-compose -f docker-compose.yml up -d
	@echo "$(GREEN)生产环境部署完成！$(NC)"

# ==========================================
# 帮助信息
# ==========================================

.PHONY: help
help:
	@echo "$(BLUE)Asura 项目 Makefile 命令列表$(NC)"
	@echo ""
	@echo "$(YELLOW)Protobuf 相关命令:$(NC)"
	@echo "  make proto          - 生成 protobuf 文件"
	@echo "  make proto-force    - 强制重新生成所有 proto 文件"
	@echo "  make proto-clean    - 清理生成的 proto 文件"
	@echo "  make proto-mapping  - 显示当前目录映射"
	@echo "  make verify-proto   - 验证生成的 protobuf 文件"
	@echo "  make proto-net      - 生成网络相关 protobuf 文件"
	@echo "  make proto-game     - 生成游戏逻辑 protobuf 文件"
	@echo "  make proto-watch    - 监控 proto 文件变化自动重新生成"
	@echo ""
	@echo "$(YELLOW)Prometheus 部署命令:$(NC)"
	@echo "  make prometheus        - 快速部署 Prometheus（完整流程）"
	@echo "  make deploy-prometheus - 部署 Prometheus 服务"
	@echo "  make stop-prometheus   - 停止 Prometheus 服务"
	@echo "  make cleanup-prometheus - 清理 Prometheus 资源"
	@echo "  make status-prometheus  - 查看 Prometheus 服务状态"
	@echo "  make logs-prometheus    - 查看 Prometheus 服务日志"
	@echo "  make restart-prometheus - 重启 Prometheus 服务"
	@echo "  make prometheus-dev     - 开发环境部署（带热重载）"
	@echo "  make prometheus-prod    - 生产环境部署"
	@echo "  make build-app-image    - 构建应用镜像"
	@echo ""
	@echo "$(YELLOW)其他命令:$(NC)"
	@echo "  make help            - 显示此帮助信息"
	@echo "  make check-docker    - 检查 Docker 环境"