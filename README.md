# Eino RAG - 企业级智能检索增强生成系统

基于 Eino 框架构建的企业级 RAG (Retrieval-Augmented Generation) 系统，提供高性能的文档管理、向量检索和智能对话功能。

## 特性

- 🚀 **高性能架构**：基于 Golang + Gin 框架，支持高并发访问
- 📚 **知识库管理**：支持多知识库隔离，灵活的文档管理
- 🔍 **智能检索**：基于 Milvus 向量数据库的语义搜索
- 💬 **智能对话**：集成 OpenAI/Ollama，支持基于知识库的智能问答
- 🔐 **完善的权限**：JWT 认证，角色权限管理
- 📊 **管理后台**：美观的 Web 管理界面
- ⚡ **Redis 缓存**：高频访问数据缓存，提升性能

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

### 集成
- Eino AI 框架
- OpenAI API
- Ollama 本地模型
- Swagger API 文档

## 快速开始

### 环境要求

- Go 1.21+
- Docker & Docker Compose
- Make 工具

### 安装步骤

1. 克隆项目
```bash
git clone https://github.com/yourusername/eino-rag.git
cd eino-rag
```

2. 复制环境配置
```bash
cp .env.example .env
# 编辑 .env 文件，配置必要的参数
```

3. 启动依赖服务
```bash
make docker-up
```

4. 安装 Go 依赖
```bash
make install-deps
```

5. 初始化目录
```bash
make init-dirs
```

6. 运行应用
```bash
make run
```

应用将在 http://localhost:8080 启动

### 默认账号

系统启动时会自动创建初始管理员账户：

- **邮箱**: admin@eino-rag.com
- **密码**: admin123456

**重要提示**: 请在首次登录后立即修改默认密码！

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
│   └── templates/     # HTML 模板
├── docs/              # API 文档
├── docker-compose.yml # Docker 配置
└── Makefile          # 构建脚本
```

## API 文档

启动应用后，访问 http://localhost:8080/swagger/index.html 查看完整的 API 文档。

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

# Milvus 配置
MILVUS_HOST=localhost
MILVUS_PORT=19530

# OpenAI 配置（可选）
OPENAI_API_KEY=your-api-key

# RAG 配置
CHUNK_SIZE=500
CHUNK_OVERLAP=50
TOP_K=5
```

## 开发指南

### 本地开发

```bash
# 使用 air 热重载
air

# 或使用 make
make dev
```

### 运行测试

```bash
make test
```

### 构建生产版本

```bash
make build-prod
```

## 部署

### Docker 部署

1. 构建镜像
```bash
docker build -t eino-rag .
```

2. 运行容器
```bash
docker run -d -p 8080:8080 --name eino-rag eino-rag
```

### 系统要求

- 最小：2 核 CPU，4GB 内存
- 推荐：4 核 CPU，8GB 内存
- 存储：根据文档量调整，建议预留 50GB+

## 贡献指南

欢迎提交 Issue 和 Pull Request！

## 许可证

Apache License 2.0