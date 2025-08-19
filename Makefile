.PHONY: build run test clean docker-build docker-run

build:
	go build -o bin/eino-rag .

run:
	go run .

test:
	go test -v ./...

clean:
	rm -rf bin/

docker-build:
	docker build -t eino-rag:latest .

docker-run:
	docker-compose up -d

docker-stop:
	docker-compose down

docker-logs:
	docker-compose logs -f

install-ollama-model:
	docker exec -it eino-rag-ollama-1 ollama pull nomic-embed-text
	docker exec -it eino-rag-ollama-1 ollama pull llama2

# 开发环境
dev:
	air -c .air.toml

# Docker开发环境
dev-docker:
	docker-compose -f docker-compose.dev.yml up --build

dev-docker-down:
	docker-compose -f docker-compose.dev.yml down

dev-docker-logs:
	docker-compose -f docker-compose.dev.yml logs -f eino-rag-dev

# 格式化代码
fmt:
	go fmt ./...
	goimports -w .

# 代码检查
lint:
	golangci-lint run

# 初始化项目
init:
	go mod download
	go install github.com/cosmtrek/air@latest
	go install golang.org/x/tools/cmd/goimports@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
