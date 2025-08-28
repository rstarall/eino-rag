#!/bin/bash

echo "Running integration tests for document upload..."

# 等待服务启动
echo "Waiting for service to be ready..."
sleep 5

# 运行集成测试
docker compose exec -T app go test -v -tags=integration ./tests/integration -run TestDocumentUpload_DuplicateHandling

echo "Test completed!"