# Eino RAG - 企业级智能检索增强生成系统

基于 Eino 框架构建的企业级 RAG (Retrieval-Augmented Generation) 系统，提供高性能的文档管理、向量检索和智能对话功能。

## 特性

- 🚀 **高性能架构**：基于 Golang + Gin 框架，支持高并发访问
- 📚 **知识库管理**：支持多知识库隔离，灵活的文档管理
- 🔍 **智能检索**：基于 Milvus 向量数据库的语义搜索
- 💬 **智能对话**：集成 OpenAI/Ollama，支持流式对话和Markdown渲染
- 🔐 **完善的权限**：JWT 认证，角色权限管理
- 📊 **管理后台**：美观的 Web 管理界面，响应式设计
- ⚡ **Redis 缓存**：高频访问数据缓存，提升性能
- 🐳 **容器化部署**：完整的 Docker Compose 配置

## 技术栈

### 后端
- Go 1.21+
- Gin Web 框架
- Gorm ORM
- SQLite 数据库
- Redis 缓存
- Milvus 向量数据库
- JWT 认证

### 前端
- 原生 JavaScript
- 响应式 CSS
- Gin 模板引擎
- Markdown-it 渲染
- 流式聊天界面

### 集成
- Eino AI 框架
- OpenAI API
- Ollama 本地模型
- Swagger API 文档

## 快速开始

### 环境要求

- Docker & Docker Compose
- Go 1.21+ (开发环境)
- Make 工具

### 部署方式

#### 方式一：Docker Compose 完整部署（推荐）

1. 克隆项目
```bash
git clone https://github.com/rstarall/eino-rag.git
cd eino-rag
```

2. 复制环境配置
```bash
cp .env.example .env
# 编辑 .env 文件，配置必要的参数
```

3. 启动所有服务（包括依赖服务）
```bash
# 启动基础服务（Milvus、Redis、Ollama）
make docker-up

# 启动应用开发环境
docker-compose -f docker-compose.dev.yml up -d
```

应用将在 http://localhost:8088 启动

#### 方式二：本地开发模式

1. 启动依赖服务
```bash
make docker-up
```

2. 安装 Go 依赖
```bash
make install-deps
```

3. 创建必要目录
```bash
make init-dirs
```

4. 运行应用
```bash
# 普通模式
make run

# 或使用热重载开发模式
make dev
```

应用将在 http://localhost:8080 启动

### 默认账号

系统启动时会自动创建初始管理员账户：

- **邮箱**: admin@eino-rag.com
- **密码**: admin123456

**重要提示**: 请在首次登录后立即修改默认密码！

## Docker 服务说明

### 基础服务 (docker-compose.yml)
- **Milvus**: 向量数据库 (端口: 19530)
- **Redis**: 缓存服务 (端口: 6379)
- **Ollama**: 本地LLM服务 (端口: 11434)
- **etcd**: Milvus 元数据存储
- **MinIO**: Milvus 对象存储

### 应用服务 (docker-compose.dev.yml)
- **eino-rag-dev**: 应用开发环境 (端口: 8088)
  - 支持热重载
  - 调试端口: 2345
  - 自动连接到基础服务

## 项目结构

```
eino-rag/
├── cmd/server/         # 应用入口
├── internal/           # 内部包
│   ├── auth/          # 认证授权
│   ├── config/        # 配置管理
│   ├── db/            # 数据库连接
│   ├── handlers/      # HTTP 处理器
│   ├── middleware/    # 中间件
│   ├── models/        # 数据模型
│   └── services/      # 业务服务
│       ├── chat/      # 聊天服务
│       ├── document/  # 文档服务
│       └── rag/       # RAG 核心服务
├── pkg/               # 公共包
│   ├── logger/        # 日志工具
│   └── utils/         # 工具函数
├── web/               # Web 资源
│   ├── static/        # 静态文件
│   │   ├── css/      # 样式文件
│   │   ├── js/       # JavaScript 文件
│   │   └── uploads/  # 上传文件目录
│   └── templates/     # HTML 模板
├── docs/              # API 文档
├── data/              # 数据文件目录
├── logs/              # 日志目录
├── docker-compose.yml     # 基础服务配置
├── docker-compose.dev.yml # 开发环境配置
├── Dockerfile         # 生产环境镜像
├── Dockerfile.dev     # 开发环境镜像
└── Makefile          # 构建脚本
```

## API 文档

启动应用后，访问以下链接查看完整的 API 文档：
- 本地开发: http://localhost:8080/swagger/index.html
- Docker 开发: http://localhost:8088/swagger/index.html

## 核心功能

### 1. 知识库管理
- 创建、编辑、删除知识库
- 知识库文档隔离
- 支持多种文档格式（PDF、TXT、Markdown、JSON、CSV、HTML）

### 2. 文档处理
- 智能文档解析
- 语义分块策略
- 向量化索引

### 3. 智能检索
- 语义相似度搜索
- 多知识库联合检索
- 结果排序优化

### 4. 对话系统
- 基于检索的上下文增强
- 流式对话支持
- Markdown 格式渲染
- 对话历史管理
- 多轮对话支持

### 5. 系统管理
- 用户权限管理
- 系统配置
- 统计分析

## 配置说明

主要配置项（.env 文件）：

```env
# 服务器配置
SERVER_PORT=8080

# 数据库配置
DB_PATH=./data/eino-rag.db

# Redis 配置
REDIS_URL=redis://localhost:6379
REDIS_PASSWORD=
REDIS_DB=0

# Milvus 配置
MILVUS_HOST=localhost
MILVUS_PORT=19530

# OpenAI 配置（可选）
OPENAI_API_KEY=your-api-key
OPENAI_BASE_URL=
OPENAI_MODEL=gpt-3.5-turbo

# Ollama 配置（可选）
OLLAMA_URL=http://localhost:11434

# 嵌入模型配置
EMBEDDING_MODEL=bge-m3
EMBEDDING_DIMENSION=1024

# RAG 配置
CHUNK_SIZE=500
CHUNK_OVERLAP=50
TOP_K=5

# JWT 配置
JWT_SECRET=your-jwt-secret

# 日志配置
LOG_LEVEL=info
LOG_FILE=logs/app.log
```

## 开发指南

### 本地开发

```bash
# 使用 air 热重载
make dev

# 或直接运行
make run
```

### 运行测试

```bash
make test
```

### 生成 API 文档

```bash
make swagger
```

### 清理环境

```bash
# 清理构建产物
make clean

# 停止 Docker 服务
make docker-down
```

## 部署

### 开发环境部署

```bash
# 启动所有服务
make docker-up
docker-compose -f docker-compose.dev.yml up -d

# 查看日志
docker-compose -f docker-compose.dev.yml logs -f
```

### 生产环境部署

1. 构建生产镜像
```bash
make build-prod
docker build -t eino-rag:latest .
```

2. 部署到生产环境
```bash
# 根据需要修改 docker-compose.yml 配置
docker-compose up -d
```

### 系统要求

- **开发环境**：2 核 CPU，4GB 内存，20GB 存储
- **生产环境**：4 核 CPU，8GB 内存，100GB+ 存储
- **GPU支持**：Ollama 服务可选配置 NVIDIA GPU

## 常用命令

```bash
# 查看可用命令
make help

# 完整开发环境启动
make docker-up && docker-compose -f docker-compose.dev.yml up -d

# 查看服务状态
docker-compose ps
docker-compose -f docker-compose.dev.yml ps

# 查看日志
docker-compose logs -f milvus redis ollama
docker-compose -f docker-compose.dev.yml logs -f eino-rag-dev

# 进入容器调试
docker-compose -f docker-compose.dev.yml exec eino-rag-dev sh

# 重启应用服务
docker-compose -f docker-compose.dev.yml restart eino-rag-dev
```

## 故障排除

### 常见问题

1. **端口冲突**
   - 修改 docker-compose.dev.yml 中的端口映射
   - 默认应用端口：8088，避免与本地8080冲突

2. **Milvus 连接失败**
   - 确保 Milvus 服务正常启动：`docker-compose logs milvus`
   - 检查防火墙设置

3. **Redis 连接失败**
   - 检查 Redis 服务状态：`docker-compose logs redis`
   - 验证连接配置

4. **文件上传失败**
   - 确保 `web/static/uploads/` 目录存在且可写
   - 检查磁盘空间

### 日志查看

```bash
# 应用日志
docker-compose -f docker-compose.dev.yml logs -f eino-rag-dev

# 基础服务日志
docker-compose logs -f milvus redis ollama

# 本地日志文件
tail -f logs/app.log
```

## 贡献指南

欢迎提交 Issue 和 Pull Request！

### 开发流程

1. Fork 项目
2. 创建特性分支
3. 提交更改
4. 推送到分支
5. 创建 Pull Request

## 许可证

Apache License 2.0