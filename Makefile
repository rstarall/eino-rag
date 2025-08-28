.PHONY: help build run test clean docker-up docker-down install-deps swagger

# 默认目标
help:
	@echo "Available commands:"
	@echo "  make install-deps  - Install Go dependencies"
	@echo "  make build        - Build the application"
	@echo "  make run          - Run the application"
	@echo "  make test         - Run tests"
	@echo "  make clean        - Clean build artifacts"
	@echo "  make docker-up    - Start docker services (Milvus, Redis)"
	@echo "  make docker-down  - Stop docker services"
	@echo "  make swagger      - Generate swagger documentation"

# 安装依赖
install-deps:
	go mod download
	go mod tidy

# 构建应用
build:
	go build -o bin/eino-rag cmd/server/main.go

# 运行应用
run:
	go run cmd/server/main.go

# 运行测试
test:
	go test ./...

# 清理构建产物
clean:
	rm -rf bin/
	rm -rf tmp/
	rm -rf logs/

# 启动Docker服务
docker-up:
	docker-compose up -d

# 停止Docker服务
docker-down:
	docker-compose down

# 生成Swagger文档
swagger:
	swag init -g cmd/server/main.go -o docs

# 开发模式运行
dev: docker-up
	air

# 生产构建
build-prod:
	CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o bin/eino-rag cmd/server/main.go

# 创建必要目录
init-dirs:
	mkdir -p data logs web/static/uploads