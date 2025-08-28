# Eino RAG - Enterprise-Level Intelligent Retrieval-Augmented Generation System

An enterprise-level RAG (Retrieval-Augmented Generation) system built on the Eino framework, providing high-performance document management, vector retrieval, and intelligent conversation capabilities.

## Features

- 🚀 **High-Performance Architecture**: Built on Golang + Gin framework, supporting high-concurrency access
- 📚 **Knowledge Base Management**: Multi-knowledge base isolation with flexible document management
- 🔍 **Intelligent Retrieval**: Semantic search based on Milvus vector database
- 💬 **Intelligent Conversation**: Integrated with OpenAI/Ollama, supporting streaming chat and Markdown rendering
- 🔐 **Comprehensive Permissions**: JWT authentication with role-based access control
- 📊 **Admin Dashboard**: Beautiful web management interface with responsive design
- ⚡ **Redis Caching**: High-frequency data caching for improved performance
- 🐳 **Containerized Deployment**: Complete Docker Compose configuration

## Tech Stack

### Backend
- Go 1.21+
- Gin Web Framework
- Gorm ORM
- SQLite Database
- Redis Cache
- Milvus Vector Database
- JWT Authentication

### Frontend
- Vanilla JavaScript
- Responsive CSS
- Gin Template Engine
- Markdown-it Rendering
- Streaming Chat Interface

### Integrations
- Eino AI Framework
- OpenAI API
- Ollama Local Models
- Swagger API Documentation

## Quick Start

### Requirements

- Docker & Docker Compose
- Go 1.21+ (for development)
- Make tools

### Deployment Options

#### Option 1: Complete Docker Compose Deployment (Recommended)

1. Clone the project
```bash
git clone https://github.com/rstarall/eino-rag.git
cd eino-rag
```

2. Copy environment configuration
```bash
cp .env.example .env
# Edit .env file and configure necessary parameters
```

3. Start all services (including dependencies)
```bash
# Start infrastructure services (Milvus, Redis, Ollama)
make docker-up

# Start application development environment
docker-compose -f docker-compose.dev.yml up -d
```

The application will be available at http://localhost:8088

#### Option 2: Local Development Mode

1. Start dependency services
```bash
make docker-up
```

2. Install Go dependencies
```bash
make install-deps
```

3. Create necessary directories
```bash
make init-dirs
```

4. Run the application
```bash
# Normal mode
make run

# Or use hot-reload development mode
make dev
```

The application will be available at http://localhost:8080

### Default Account

The system automatically creates an initial admin account on startup:

- **Email**: admin@eino-rag.com
- **Password**: admin123456

**Important**: Please change the default password immediately after first login!

## Docker Services Overview

### Infrastructure Services (docker-compose.yml)
- **Milvus**: Vector database (Port: 19530)
- **Redis**: Cache service (Port: 6379)
- **Ollama**: Local LLM service (Port: 11434)
- **etcd**: Milvus metadata storage
- **MinIO**: Milvus object storage

### Application Service (docker-compose.dev.yml)
- **eino-rag-dev**: Application development environment (Port: 8088)
  - Hot reload support
  - Debug port: 2345
  - Auto-connects to infrastructure services

## Project Structure

```
eino-rag/
├── cmd/server/         # Application entry point
├── internal/           # Internal packages
│   ├── auth/          # Authentication & authorization
│   ├── config/        # Configuration management
│   ├── db/            # Database connections
│   ├── handlers/      # HTTP handlers
│   ├── middleware/    # Middleware
│   ├── models/        # Data models
│   └── services/      # Business services
│       ├── chat/      # Chat service
│       ├── document/  # Document service
│       └── rag/       # RAG core service
├── pkg/               # Public packages
│   ├── logger/        # Logging utilities
│   └── utils/         # Utility functions
├── web/               # Web resources
│   ├── static/        # Static files
│   │   ├── css/      # Stylesheets
│   │   ├── js/       # JavaScript files
│   │   └── uploads/  # Upload directory
│   └── templates/     # HTML templates
├── docs/              # API documentation
├── data/              # Data files directory
├── logs/              # Log directory
├── docker-compose.yml     # Infrastructure services config
├── docker-compose.dev.yml # Development environment config
├── Dockerfile         # Production image
├── Dockerfile.dev     # Development image
└── Makefile          # Build scripts
```

## API Documentation

After starting the application, visit the following links to view complete API documentation:
- Local development: http://localhost:8080/swagger/index.html
- Docker development: http://localhost:8088/swagger/index.html

## Core Features

### 1. Knowledge Base Management
- Create, edit, and delete knowledge bases
- Knowledge base document isolation
- Support for multiple document formats (PDF, TXT, Markdown, JSON, CSV, HTML)

### 2. Document Processing
- Intelligent document parsing
- Semantic chunking strategies
- Vector indexing

### 3. Intelligent Retrieval
- Semantic similarity search
- Multi-knowledge base joint retrieval
- Result ranking optimization

### 4. Chat System
- Retrieval-based context enhancement
- Streaming chat support
- Markdown format rendering
- Conversation history management
- Multi-turn conversation support

### 5. System Management
- User permission management
- System configuration
- Statistical analysis

## Configuration

Main configuration items (.env file):

```env
# Server configuration
SERVER_PORT=8080

# Database configuration
DB_PATH=./data/eino-rag.db

# Redis configuration
REDIS_URL=redis://localhost:6379
REDIS_PASSWORD=
REDIS_DB=0

# Milvus configuration
MILVUS_HOST=localhost
MILVUS_PORT=19530

# OpenAI configuration (optional)
OPENAI_API_KEY=your-api-key
OPENAI_BASE_URL=
OPENAI_MODEL=gpt-3.5-turbo

# Ollama configuration (optional)
OLLAMA_URL=http://localhost:11434

# Embedding model configuration
EMBEDDING_MODEL=bge-m3
EMBEDDING_DIMENSION=1024

# RAG configuration
CHUNK_SIZE=500
CHUNK_OVERLAP=50
TOP_K=5

# JWT configuration
JWT_SECRET=your-jwt-secret

# Logging configuration
LOG_LEVEL=info
LOG_FILE=logs/app.log
```

## Development Guide

### Local Development

```bash
# Use air hot reload
make dev

# Or run directly
make run
```

### Run Tests

```bash
make test
```

### Generate API Documentation

```bash
make swagger
```

### Clean Environment

```bash
# Clean build artifacts
make clean

# Stop Docker services
make docker-down
```

## Deployment

### Development Environment Deployment

```bash
# Start all services
make docker-up
docker-compose -f docker-compose.dev.yml up -d

# View logs
docker-compose -f docker-compose.dev.yml logs -f
```

### Production Environment Deployment

1. Build production image
```bash
make build-prod
docker build -t eino-rag:latest .
```

2. Deploy to production
```bash
# Modify docker-compose.yml configuration as needed
docker-compose up -d
```

### System Requirements

- **Development Environment**: 2 CPU cores, 4GB RAM, 20GB storage
- **Production Environment**: 4 CPU cores, 8GB RAM, 100GB+ storage
- **GPU Support**: Optional NVIDIA GPU configuration for Ollama service

## Common Commands

```bash
# View available commands
make help

# Complete development environment startup
make docker-up && docker-compose -f docker-compose.dev.yml up -d

# Check service status
docker-compose ps
docker-compose -f docker-compose.dev.yml ps

# View logs
docker-compose logs -f milvus redis ollama
docker-compose -f docker-compose.dev.yml logs -f eino-rag-dev

# Enter container for debugging
docker-compose -f docker-compose.dev.yml exec eino-rag-dev sh

# Restart application service
docker-compose -f docker-compose.dev.yml restart eino-rag-dev
```

## Troubleshooting

### Common Issues

1. **Port Conflicts**
   - Modify port mappings in docker-compose.dev.yml
   - Default application port: 8088, avoiding conflicts with local 8080

2. **Milvus Connection Failure**
   - Ensure Milvus service is running properly: `docker-compose logs milvus`
   - Check firewall settings

3. **Redis Connection Failure**
   - Check Redis service status: `docker-compose logs redis`
   - Verify connection configuration

4. **File Upload Failure**
   - Ensure `web/static/uploads/` directory exists and is writable
   - Check disk space

### Log Viewing

```bash
# Application logs
docker-compose -f docker-compose.dev.yml logs -f eino-rag-dev

# Infrastructure service logs
docker-compose logs -f milvus redis ollama

# Local log files
tail -f logs/app.log
```

## Contributing

Issues and Pull Requests are welcome!

### Development Workflow

1. Fork the project
2. Create a feature branch
3. Commit your changes
4. Push to the branch
5. Create a Pull Request

## License

Apache License 2.0
