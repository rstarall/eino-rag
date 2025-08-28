package handlers

import "time"

// Common response types

type ErrorResponse struct {
	Success bool   `json:"success" example:"false"`
	Message string `json:"message" example:"Error message"`
}

type SuccessResponse struct {
	Success bool   `json:"success" example:"true"`
	Message string `json:"message" example:"Operation successful"`
}

// Upload response types

type UploadResponse struct {
	Success    bool   `json:"success" example:"true"`
	Message    string `json:"message" example:"Document indexed successfully"`
	DocumentID uint   `json:"document_id,omitempty" example:"123"`
	ChunkCount int    `json:"chunk_count,omitempty" example:"5"`
}

// Search request/response types

type SearchRequest struct {
	Query           string `json:"query" binding:"required" example:"人工智能的发展历史"`
	KnowledgeBaseID uint   `json:"kb_id,omitempty" example:"1"`
	TopK            int    `json:"top_k,omitempty" example:"5"`
	ReturnContext   bool   `json:"return_context" example:"true"`
}

type SearchResponse struct {
	Success   bool        `json:"success" example:"true"`
	Query     string      `json:"query" example:"人工智能的发展历史"`
	Context   string      `json:"context,omitempty" example:"根据检索到的文档..."`
	Documents []DocResult `json:"documents"`
	Timestamp int64       `json:"timestamp" example:"1640995200"`
}

type DocResult struct {
	ID       string                 `json:"id" example:"doc_12345"`
	Content  string                 `json:"content" example:"这是文档的内容片段..."`
	Score    float64                `json:"score" example:"0.85"`
	Metadata map[string]interface{} `json:"metadata"`
}

// Chat request/response types

type ChatRequest struct {
	Message         string `json:"message" binding:"required" example:"你好，请介绍一下人工智能"`
	ConversationID  string `json:"conversation_id,omitempty" example:"550e8400-e29b-41d4-a716-446655440000"`
	KnowledgeBaseID uint   `json:"kb_id,omitempty" example:"1"`
	UseRAG          bool   `json:"use_rag" example:"true"`
}

type ChatResponse struct {
	Success        bool   `json:"success" example:"true"`
	Message        string `json:"message" example:"AI的回复内容"`
	ConversationID string `json:"conversation_id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Context        string `json:"context,omitempty" example:"基于以下文档..."`
	Timestamp      int64  `json:"timestamp" example:"1640995200"`
}

// Knowledge base types

type CreateKBRequest struct {
	Name        string `json:"name" binding:"required,min=1,max=200" example:"技术文档库"`
	Description string `json:"description" example:"存储技术相关文档"`
}

type UpdateKBRequest struct {
	Name        string `json:"name,omitempty" example:"更新后的名称"`
	Description string `json:"description,omitempty" example:"更新后的描述"`
}

type KBListResponse struct {
	Success         bool                    `json:"success" example:"true"`
	KnowledgeBases  []KnowledgeBaseWithDocs `json:"knowledge_bases"`
	Total           int64                   `json:"total" example:"10"`
	Page            int                     `json:"page" example:"1"`
	PageSize        int                     `json:"page_size" example:"10"`
}

type KnowledgeBaseWithDocs struct {
	ID          uint      `json:"id" example:"1"`
	Name        string    `json:"name" example:"技术文档库"`
	Description string    `json:"description" example:"存储技术相关文档"`
	DocCount    int       `json:"doc_count" example:"42"`
	CreatorID   uint      `json:"creator_id" example:"1"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Document types

type DocumentListResponse struct {
	Success   bool            `json:"success" example:"true"`
	Documents []DocumentInfo  `json:"documents"`
	Total     int64           `json:"total" example:"50"`
	Page      int             `json:"page" example:"1"`
	PageSize  int             `json:"page_size" example:"10"`
}

type DocumentInfo struct {
	ID              uint      `json:"id" example:"123"`
	KnowledgeBaseID uint      `json:"kb_id" example:"1"`
	KnowledgeBaseName string  `json:"kb_name,omitempty" example:"技术文档"`
	FileName        string    `json:"file_name" example:"document.pdf"`
	FileSize        int64     `json:"file_size" example:"1048576"`
	Hash            string    `json:"hash" example:"abc123..."`
	CreatorID       uint      `json:"creator_id" example:"1"`
	CreatedAt       time.Time `json:"created_at"`
}

// System config types

type SystemConfigRequest struct {
	Configs map[string]interface{} `json:"configs" binding:"required"`
}

type SystemConfigResponse struct {
	Success bool                   `json:"success" example:"true"`
	Configs map[string]interface{} `json:"configs"`
}

// Health check

type HealthResponse struct {
	Status    string `json:"status" example:"healthy"`
	Timestamp int64  `json:"timestamp" example:"1640995200"`
	Service   string `json:"service" example:"eino-rag"`
	Version   string `json:"version" example:"1.0.0"`
}