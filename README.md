# Eino RAG 项目

基于 CloudWeGo Eino 框架构建的检索增强生成(RAG)系统，使用 Milvus 向量数据库和 Ollama 本地大语言模型。

## 功能特性

- 🔍 **智能文档检索**：基于语义相似度的文档检索
- 📄 **智能文档处理**：基于语义相似度的智能分块和嵌入生成
- 🚀 **高性能**：使用 Milvus 向量数据库进行快速相似性搜索
- 🤖 **本地部署**：集成 Ollama 本地大语言模型
- 🤖 **OpenAI 集成**：基于 CloudWeGo Eino 的 OpenAI 流式对话
- 🐳 **容器化**：完整的 Docker 部署方案
- 🎯 **RESTful API**：简洁的 HTTP 接口
- 🌐 **Web界面**：现代化的聊天式Web界面
- ⚡ **实时流式响应**：支持SSE流式对话
- 🔄 **热重载开发**：开发环境自动热重载

## 系统架构

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   用户查询      │────▶│   RAG 服务      │────▶│   文档检索      │
└─────────────────┘    └─────────────────┘    └─────────────────┘
                                │
                                ▼
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   结果返回      │◀────│   上下文构建    │    │   Milvus 存储   │
└─────────────────┘    └─────────────────┘    └─────────────────┘
```

## 项目结构

```
eino-rag/
├── main.go                         # 主程序入口
├── go.mod                          # Go 模块定义
├── config/                         # 配置管理
│   └── config.go
├── components/                     # 核心组件
│   ├── ollama_embedding.go         # Ollama 嵌入服务
│   ├── milvus_retriever.go         # Milvus 检索器
│   └── document_processor.go       # 文档处理器
├── services/                       # 业务服务层
│   └── rag_service.go              # RAG 核心服务
├── handlers/                       # HTTP 处理器
│   ├── api_handlers.go             # API 处理
│   └── frontend_handlers.go        # 前端和 SSE 处理
├── frontend/                       # 前端文件
│   ├── index.html                  # 主页面
│   └── static/
│       ├── css/
│       │   └── style.css           # 样式文件
│       └── js/
│           └── app.js              # 前端逻辑
├── docker-compose.yml              # 生产环境Docker编排
├── docker-compose.dev.yml          # 开发环境Docker编排
├── Dockerfile                      # 生产环境容器构建文件
├── Dockerfile.dev                  # 开发环境容器构建文件
├── .air.toml                       # Air热重载配置
├── .env.example                    # 环境变量配置示例
├── .gitignore                      # Git忽略文件
├── SETUP.md                        # 项目设置指南
├── UPGRADE.md                      # 升级指南：语义分割器
├── PERFORMANCE.md                  # 性能分析：语义分割延迟
└── Makefile                        # 构建脚本
```

## 快速开始

⚠️ **重要提示：在开始之前，请先阅读 [SETUP.md](SETUP.md) 文件了解如何正确安装 CloudWeGo Eino 依赖。**

### 方法一：Docker 部署（推荐）

1. **克隆项目并配置环境**
```bash
git clone <repository>
cd eino-rag
cp .env.example .env
# 编辑 .env 文件，特别是设置 OPENAI_API_KEY
```

2. **安装依赖**
```bash
# 请参考 SETUP.md 安装 Eino 依赖
go get github.com/cloudwego/eino@latest
go get github.com/cloudwego/eino-ext@latest
go mod tidy
```

3. **启动服务**
```bash
make docker-run
```

3. **安装 Ollama 模型**
```bash
make install-ollama-model
```

3. **访问Web界面**
```bash
# 浏览器访问
http://localhost:8080

# 或验证API服务
curl http://localhost:8080/api/v1/health
```

### 方法二：本地开发

1. **环境要求**
   - Go 1.23+
   - Milvus 2.3+
   - Ollama

2. **安装依赖**
```bash
make init
go mod download
```

3. **配置环境变量**
```bash
# 复制配置文件模板
cp .env.example .env

# 编辑配置文件
vim .env
```

4. **运行服务**
```bash
make run
# 或开发模式（热重载）
make dev
```

### 方法三：开发环境（热重载）

1. **配置并启动Docker开发环境**
```bash
cp .env.example .env
# 编辑 .env 文件配置
make dev-docker
```

2. **本地开发环境**
```bash
make init
make dev
```

3. **访问服务**
```bash
# Web界面
http://localhost:8080

# 调试端口（如果需要）
http://localhost:2345
```

## Web 界面

访问 `http://localhost:8080` 即可使用现代化的聊天界面：

### 主要功能
- 🖱️ **拖拽上传**：支持拖拽或点击上传文档
- 💬 **流式对话**：实时流式响应，模拟人类对话
- 📊 **上下文展示**：可选择显示检索到的文档片段
- ⚙️ **参数调节**：可调节Top-K等检索参数
- 📤 **对话导出**：支持导出对话历史
- 📱 **响应式设计**：支持移动端访问

### 支持的文件格式
- 纯文本文件 (`.txt`)
- Markdown文件 (`.md`)
- PDF文档 (`.pdf`)
- Word文档 (`.doc`, `.docx`)

## API 接口

### 1. 健康检查
```http
GET /api/v1/health
```

**响应示例：**
```json
{
  "status": "healthy",
  "timestamp": 1703123456,
  "service": "eino-rag"
}
```

### 2. 文档上传
```http
POST /api/v1/upload
Content-Type: multipart/form-data

file: [文档文件]
metadata: [可选的JSON元数据]
```

**响应示例：**
```json
{
  "success": true,
  "message": "Document indexed successfully"
}
```

### 3. 文档检索
```http
POST /api/v1/search
Content-Type: application/json

{
  "query": "查询内容",
  "return_context": true
}
```

**响应示例：**
```json
{
  "success": true,
  "query": "查询内容",
  "context": "相关上下文信息...",
  "documents": [
    {
      "id": "doc_123",
      "content": "文档内容片段",
      "score": 0.95,
      "metadata": {
        "filename": "example.txt",
        "chunk_index": 0
      }
    }
  ],
  "timestamp": 1703123456
}
```

## 配置说明

复制 `.env.example` 为 `.env` 文件并根据需要修改配置。系统支持通过环境变量进行配置：

| 环境变量 | 默认值 | 说明 |
|---------|--------|------|
| `MILVUS_HOST` | localhost | Milvus 服务地址 |
| `MILVUS_PORT` | 19530 | Milvus 服务端口 |
| `COLLECTION_NAME` | rag_documents | 集合名称 |
| `VECTOR_DIM` | 768 | 向量维度 |
| `OLLAMA_URL` | http://localhost:11434 | Ollama 服务地址 |
| `EMBEDDING_MODEL` | nomic-embed-text | 嵌入模型 |
| `OPENAI_API_KEY` | - | OpenAI API 密钥（可选） |
| `OPENAI_MODEL` | gpt-3.5-turbo | OpenAI 模型名称 |
| `OPENAI_BASE_URL` | - | OpenAI API 基础URL（可选） |
| `CHUNK_SIZE` | 500 | 文档最小分块大小 |
| `CHUNK_OVERLAP` | 50 | 分块重叠大小（已废弃） |
| `TOP_K` | 5 | 检索结果数量 |
| `SEMANTIC_SPLITTING` | true | 是否启用语义分割 |
| `SMALL_DOC_THRESHOLD` | 800 | 小文档阈值（字符数） |
| `EMBEDDING_CACHE` | true | 是否启用嵌入缓存 |
| `SERVER_PORT` | 8080 | 服务端口 |

## 开发指南

### 环境配置

```bash
# 复制环境变量模板
cp .env.example .env

# 根据需要编辑配置
nano .env  # 或使用其他编辑器
```

### 构建命令

```bash
# 编译项目
make build

# 运行项目
make run

# 运行测试
make test

# 代码格式化
make fmt

# 代码检查
make lint

# 清理构建文件
make clean
```

### Docker 命令

```bash
# 构建镜像
make docker-build

# 生产环境
make docker-run        # 启动生产环境
make docker-stop       # 停止服务
make docker-logs       # 查看日志

# 开发环境
make dev-docker        # 启动开发环境（支持热重载）
make dev-docker-down   # 停止开发环境
make dev-docker-logs   # 查看开发环境日志
```

## 技术栈

- **框架**：CloudWeGo Eino - 云原生AI编排框架
- **向量数据库**：Milvus - 高性能向量相似性搜索
- **嵌入模型**：Ollama - 本地部署的嵌入服务
- **Web框架**：Gin - 高性能HTTP框架
- **日志**：Zap - 结构化日志库
- **容器化**：Docker & Docker Compose

## 性能特性

- **高并发**：支持大量并发请求处理
- **低延迟**：优化的向量检索算法
- **可扩展**：模块化设计，易于扩展功能
- **容错性**：完善的错误处理机制

## 贡献指南

1. Fork 本项目
2. 创建特性分支 (`git checkout -b feature/amazing-feature`)
3. 提交更改 (`git commit -m 'Add some amazing feature'`)
4. 推送到分支 (`git push origin feature/amazing-feature`)
5. 提交 Pull Request

## 许可证

本项目采用 MIT 许可证，详情请查看 [LICENSE](LICENSE) 文件。

## 问题反馈

如有问题或建议，请提交 [Issue](issues) 或联系维护者。

---

**注意**：首次运行需要下载 Ollama 模型，可能需要一些时间。建议使用 Docker 部署以获得最佳体验。
