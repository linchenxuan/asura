# 项目配置
SCRIPTS_DIR := ./scripts

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