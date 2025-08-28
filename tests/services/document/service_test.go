package document_test

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"eino-rag/internal/db"
	"eino-rag/internal/models"
	"eino-rag/internal/services/document"
)

type MockParser struct {
	mock.Mock
}

func (m *MockParser) ParseDocument(filename string, data []byte) (string, error) {
	args := m.Called(filename, data)
	return args.String(0), args.Error(1)
}

type MockChunker struct {
	mock.Mock
}

func (m *MockChunker) ChunkText(text string, metadata map[string]interface{}) ([]*models.DocumentChunk, error) {
	args := m.Called(text, metadata)
	return args.Get(0).([]*models.DocumentChunk), args.Error(1)
}

type MockEmbedder struct {
	mock.Mock
}

func (m *MockEmbedder) EmbedDocuments(chunks []*models.DocumentChunk) error {
	args := m.Called(chunks)
	return args.Error(0)
}

func setupTestDB(t *testing.T) (sqlmock.Sqlmock, func()) {
	mockDB, mock, err := sqlmock.New()
	assert.NoError(t, err)

	dialector := mysql.New(mysql.Config{
		Conn:                      mockDB,
		SkipInitializeWithVersion: true,
	})

	gormDB, err := gorm.Open(dialector, &gorm.Config{})
	assert.NoError(t, err)

	db.SetDB(gormDB)

	cleanup := func() {
		mockDB.Close()
	}

	return mock, cleanup
}

func TestUploadDocument_NewDocument(t *testing.T) {
	mock, cleanup := setupTestDB(t)
	defer cleanup()

	logger := zap.NewNop()
	mockParser := new(MockParser)
	mockChunker := new(MockChunker)
	mockEmbedder := new(MockEmbedder)

	service := document.NewService(logger, mockParser, mockChunker, mockEmbedder)

	filename := "test.pdf"
	kbID := uint(1)
	userID := uint(1)
	fileData := []byte("test content")
	hash := fmt.Sprintf("%x", sha256.Sum256(fileData))

	// 期望的查询：检查文档是否已存在
	mock.ExpectQuery("SELECT \\* FROM `documents`").
		WithArgs(hash, kbID).
		WillReturnRows(sqlmock.NewRows([]string{})) // 返回空结果，模拟 record not found

	// 期望的解析操作
	mockParser.On("ParseDocument", filename, fileData).Return("parsed text content", nil)

	// 期望开始事务
	mock.ExpectBegin()

	// 期望插入文档
	mock.ExpectExec("INSERT INTO `documents`").
		WithArgs(kbID, filename, int64(len(fileData)), hash, userID, sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	// 模拟chunk操作
	chunks := []*models.DocumentChunk{
		{
			DocumentID: 1,
			ChunkIndex: 0,
			Content:    "chunk 1",
		},
	}
	mockChunker.On("ChunkText", "parsed text content", mock.Anything).Return(chunks, nil)

	// 期望批量插入chunks
	mock.ExpectExec("INSERT INTO `document_chunks`").
		WillReturnResult(sqlmock.NewResult(1, 1))

	// 模拟embedding操作
	mockEmbedder.On("EmbedDocuments", chunks).Return(nil)

	// 期望更新chunks的embedding状态
	mock.ExpectExec("UPDATE `document_chunks`").
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))

	// 期望提交事务
	mock.ExpectCommit()

	// 执行测试
	doc, chunkCount, err := service.UploadDocument(filename, bytes.NewReader(fileData), kbID, userID)

	// 验证结果
	assert.NoError(t, err)
	assert.NotNil(t, doc)
	assert.Equal(t, filename, doc.FileName)
	assert.Equal(t, hash, doc.Hash)
	assert.Equal(t, 1, chunkCount)

	// 验证所有期望都被满足
	assert.NoError(t, mock.ExpectationsWereMet())
	mockParser.AssertExpectations(t)
	mockChunker.AssertExpectations(t)
	mockEmbedder.AssertExpectations(t)
}

func TestUploadDocument_DuplicateDocument(t *testing.T) {
	mock, cleanup := setupTestDB(t)
	defer cleanup()

	logger := zap.NewNop()
	mockParser := new(MockParser)
	mockChunker := new(MockChunker)
	mockEmbedder := new(MockEmbedder)

	service := document.NewService(logger, mockParser, mockChunker, mockEmbedder)

	filename := "test.pdf"
	kbID := uint(1)
	userID := uint(1)
	fileData := []byte("test content")
	hash := fmt.Sprintf("%x", sha256.Sum256(fileData))

	// 期望的查询：检查文档是否已存在
	rows := sqlmock.NewRows([]string{"id", "knowledge_base_id", "file_name", "hash"}).
		AddRow(1, kbID, filename, hash)
	mock.ExpectQuery("SELECT \\* FROM `documents`").
		WithArgs(hash, kbID).
		WillReturnRows(rows) // 返回结果，表示文档已存在

	// 执行测试
	doc, chunkCount, err := service.UploadDocument(filename, bytes.NewReader(fileData), kbID, userID)

	// 验证结果
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "document already exists in this knowledge base")
	assert.Nil(t, doc)
	assert.Equal(t, 0, chunkCount)

	// 验证所有期望都被满足
	assert.NoError(t, mock.ExpectationsWereMet())
}