## 设计目标

Eino-RAG 设计目标是成为一个企业级高并发的RAG系统，完善的后台管理机制

## 技术选型
  - Golang、Gin、Sqlite
  - Eino、Milvus、Redis、Ollama
  - html、js、css，Gin template
  
## 核心功能
RAG系统:
  - 高并发相关文档检索(Mivlus)
  - 支持外接第三方知识库API
  - 支持User Key鉴权
  - 基于Redis的高频访问文档向量缓存
  - 基于Gin的高性能Rest API(支持swagger文档)
  
RAG管理后端:
  - (使用Gorm、Sqlite保存数据，并使用Redis进行缓存高频数据)
  - 授权登录管理、身份认证
  - 知识库管理(增删改查)
  
前端功能:
  - 基于RAG的聊天Demo
  - RAG后台管理系统(Sqlite+Redis)