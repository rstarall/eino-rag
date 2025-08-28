## Sqlite数据库表定义
### 用户表(user_table)
记录用户信息:
>id、名称(name)、邮箱、密码hash、登录时间、User Token(用于接口访问授权,加在请求头Authorization:Bearer xxxx)、用户角色名称(role_name)

### 角色权限表(role_table)
用户角色权限:
>id、名称(name)、权限等级(0最高)、功能开启列表(varchar json list)

### 知识库表(kg_table)
存放知识库的信息
>id、名称(name)、文档数量、库描述、创建时间、创建者id

### 文档表(doc_tbale)
存放所有文档条目
>id、知识库ID(kg_id)、文件名、文件大小、hash、创建时间、创建者id

### Chat对话记录表
存放用户对话历史
>id、用户ID(user_id)、对话id(uu4id->内容存放在redis)、创建时间

### 系统配置表
存放系统的配置(Milvus 向量数据库配置、Ollama 配置、OpenAI 配置、RAG配置)
>根据.env的配置创建,表分2列(key、value),字符串类型