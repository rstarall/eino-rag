package document

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"time"

	"eino-rag/internal/config"
	"eino-rag/internal/db"
	"eino-rag/internal/models"
	"eino-rag/internal/services/rag"

	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type Service struct {
	parser    *DocumentParser
	processor *DocumentProcessor
	retriever *rag.MilvusRetriever
	logger    *zap.Logger
	config    *config.Config
}

func NewService(
	parser *DocumentParser,
	processor *DocumentProcessor,
	retriever *rag.MilvusRetriever,
	cfg *config.Config,
	logger *zap.Logger,
) *Service {
	return &Service{
		parser:    parser,
		processor: processor,
		retriever: retriever,
		logger:    logger,
		config:    cfg,
	}
}

// UploadDocument 上传并处理文档
func (s *Service) UploadDocument(
	ctx context.Context,
	filename string,
	content io.Reader,
	kbID uint,
	userID uint,
) (*models.Document, int, error) {
	// 先检查retriever是否可用
	if s.retriever == nil {
		return nil, 0, fmt.Errorf("vector database is not available, please try again later")
	}
	
	// 验证知识库是否存在
	database := db.GetDB()
	var kb models.KnowledgeBase
	if err := database.First(&kb, kbID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, 0, fmt.Errorf("knowledge base with id %d not found", kbID)
		}
		return nil, 0, fmt.Errorf("failed to check knowledge base: %w", err)
	}
	// Debug: Log allowed file types
	s.logger.Info("Validating file upload",
		zap.String("filename", filename),
		zap.Strings("allowed_types", s.config.AllowedFileTypes))
	
	// 验证文件类型
	if err := s.parser.ValidateFileType(filename, s.config.AllowedFileTypes); err != nil {
		return nil, 0, err
	}

	// 读取文件内容
	data, err := io.ReadAll(io.LimitReader(content, s.config.MaxUploadSize))
	if err != nil {
		return nil, 0, fmt.Errorf("failed to read file: %w", err)
	}

	// 计算文件哈希
	hash := fmt.Sprintf("%x", sha256.Sum256(data))

	// 检查文件是否已存在
	database = db.GetDB()
	var existingDoc models.Document
	if err := database.Where("hash = ? AND knowledge_base_id = ?", hash, kbID).First(&existingDoc).Error; err == nil {
		return nil, 0, fmt.Errorf("document already exists in this knowledge base")
	}

	// 解析文档内容
	text, err := s.parser.ParseDocument(filename, data)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to parse document: %w", err)
	}

	// 创建文档记录
	doc := &models.Document{
		KnowledgeBaseID: kbID,
		FileName:        filename,
		FileSize:        int64(len(data)),
		Hash:            hash,
		CreatorID:       userID,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	// 开始事务
	chunkCount := 0
	var chunks []*schema.Document
	err = database.Transaction(func(tx *gorm.DB) error {
		// 保存文档记录
		if err := tx.Create(doc).Error; err != nil {
			return fmt.Errorf("failed to save document: %w", err)
		}

		// 处理文档内容为chunks
		s.logger.Info("Starting document processing",
			zap.String("filename", filename),
			zap.Uint("doc_id", doc.ID),
			zap.Int("text_length", len(text)))
		
		metadata := map[string]interface{}{
			"filename": filename,
			"kb_id":    kbID,
			"doc_id":   doc.ID,
			"user_id":  userID,
		}

		// 使用 goroutine 和超时处理文本处理
		type processResult struct {
			chunks []*schema.Document
			err    error
		}
		
		resultChan := make(chan processResult, 1)
		
		go func() {
			chunks, err := s.processor.ProcessText(text, metadata)
			resultChan <- processResult{chunks: chunks, err: err}
		}()
		
		// 使用配置的索引超时
		select {
		case result := <-resultChan:
			if result.err != nil {
				return fmt.Errorf("failed to process document: %w", result.err)
			}
			chunks = result.chunks
		case <-time.After(s.config.IndexTimeout):
			return fmt.Errorf("document processing timeout after %v", s.config.IndexTimeout)
		}

		chunkCount = len(chunks)
		s.logger.Info("Document processed into chunks",
			zap.String("filename", filename),
			zap.Uint("doc_id", doc.ID),
			zap.Int("chunk_count", chunkCount))

		// 添加到向量数据库
		s.logger.Info("Starting vector indexing",
			zap.String("filename", filename),
			zap.Uint("doc_id", doc.ID),
			zap.Int("chunk_count", chunkCount))
		
		if err := s.retriever.AddDocuments(ctx, chunks, kbID, doc.ID); err != nil {
			return fmt.Errorf("failed to index document: %w", err)
		}
		
		s.logger.Info("Vector indexing completed",
			zap.String("filename", filename),
			zap.Uint("doc_id", doc.ID))

		// 更新知识库文档数量
		s.logger.Info("Updating knowledge base doc count",
			zap.Uint("kb_id", kbID))
		
		// 使用 Exec 执行原生 SQL 更新
		result := tx.Exec("UPDATE knowledge_bases SET doc_count = doc_count + 1, updated_at = ? WHERE id = ?", 
			time.Now(), kbID)
		
		if result.Error != nil {
			return fmt.Errorf("failed to update knowledge base doc count: %w", result.Error)
		}
		
		if result.RowsAffected == 0 {
			return fmt.Errorf("knowledge base with id %d not found", kbID)
		}
		
		s.logger.Info("Knowledge base doc count updated",
			zap.Uint("kb_id", kbID),
			zap.Int64("rows_affected", result.RowsAffected))

		return nil
	})

	if err != nil {
		return nil, 0, err
	}

	s.logger.Info("Document uploaded successfully",
		zap.String("filename", filename),
		zap.Uint("kb_id", kbID),
		zap.Uint("doc_id", doc.ID),
		zap.Int("chunks", chunkCount))

	return doc, chunkCount, nil
}

// SearchDocuments 搜索文档
func (s *Service) SearchDocuments(ctx context.Context, query string, kbID uint, topK int) ([]*schema.Document, error) {
	if s.retriever == nil {
		return nil, fmt.Errorf("vector search is not available - Milvus connection failed")
	}
	
	if topK <= 0 {
		topK = s.config.TopK
	}

	// 使用检索器搜索
	docs, err := s.retriever.Retrieve(ctx, query, kbID)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve documents: %w", err)
	}

	// 限制返回数量
	if len(docs) > topK {
		docs = docs[:topK]
	}

	return docs, nil
}

// DeleteDocument 删除文档
func (s *Service) DeleteDocument(ctx context.Context, docID uint) error {
	database := db.GetDB()

	var doc models.Document
	if err := database.First(&doc, docID).Error; err != nil {
		return fmt.Errorf("document not found: %w", err)
	}

	// 开始事务
	return database.Transaction(func(tx *gorm.DB) error {
		// 从向量数据库删除
		if s.retriever != nil {
			if err := s.retriever.DeleteByDocument(ctx, docID); err != nil {
				return fmt.Errorf("failed to delete from vector database: %w", err)
			}
		} else {
			s.logger.Warn("Vector deletion skipped - retriever not available",
				zap.Uint("doc_id", docID))
		}

		// 删除数据库记录
		if err := tx.Delete(&doc).Error; err != nil {
			return fmt.Errorf("failed to delete document record: %w", err)
		}

		// 更新知识库文档数量
		if err := tx.Model(&models.KnowledgeBase{}).
			Where("id = ?", doc.KnowledgeBaseID).
			Update("doc_count", gorm.Expr("doc_count - 1")).Error; err != nil {
			return fmt.Errorf("failed to update knowledge base doc count: %w", err)
		}

		return nil
	})
}

// GetDocumentsByKB 获取知识库的文档列表
func (s *Service) GetDocumentsByKB(kbID uint, page, pageSize int) ([]models.Document, int64, error) {
	database := db.GetDB()

	var total int64
	var docs []models.Document

	// 计算总数
	if err := database.Model(&models.Document{}).Where("knowledge_base_id = ?", kbID).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 分页查询
	offset := (page - 1) * pageSize
	if err := database.Where("knowledge_base_id = ?", kbID).
		Offset(offset).
		Limit(pageSize).
		Order("created_at DESC").
		Find(&docs).Error; err != nil {
		return nil, 0, err
	}

	return docs, total, nil
}

// GetAllDocuments 获取所有文档（支持分页）
func (s *Service) GetAllDocuments(page, pageSize int) ([]models.Document, int64, error) {
	database := db.GetDB()

	var total int64
	var docs []models.Document

	// 计算总数
	if err := database.Model(&models.Document{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 分页查询，预加载知识库信息
	offset := (page - 1) * pageSize
	if err := database.Preload("KnowledgeBase").
		Offset(offset).
		Limit(pageSize).
		Order("created_at DESC").
		Find(&docs).Error; err != nil {
		return nil, 0, err
	}

	return docs, total, nil
}