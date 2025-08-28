// +build integration

package integration_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"testing"
	"time"
)

func TestDocumentCount_Update(t *testing.T) {
	// 获取token
	token := loginAndGetToken(t)

	// 创建知识库
	kbID := createKnowledgeBase(t, token)
	
	// 获取初始的知识库信息
	initialKB := getKnowledgeBase(t, token, kbID)
	initialDocCount := int(initialKB["doc_count"].(float64))
	t.Logf("Initial doc count: %d", initialDocCount)

	// 上传第一个文档
	uploadDocument(t, token, kbID, "doc1.pdf", []byte(testPDF))
	
	// 验证文档数量增加
	kb1 := getKnowledgeBase(t, token, kbID)
	docCount1 := int(kb1["doc_count"].(float64))
	t.Logf("Doc count after first upload: %d", docCount1)
	
	if docCount1 != initialDocCount+1 {
		t.Errorf("Expected doc count to be %d, got %d", initialDocCount+1, docCount1)
	}

	// 修改PDF内容使其不同
	testPDF2 := testPDF + "\n% Modified content"
	
	// 上传第二个文档
	uploadDocument(t, token, kbID, "doc2.pdf", []byte(testPDF2))
	
	// 再次验证文档数量
	kb2 := getKnowledgeBase(t, token, kbID)
	docCount2 := int(kb2["doc_count"].(float64))
	t.Logf("Doc count after second upload: %d", docCount2)
	
	if docCount2 != initialDocCount+2 {
		t.Errorf("Expected doc count to be %d, got %d", initialDocCount+2, docCount2)
	}
}

func getKnowledgeBase(t *testing.T, token string, kbID uint) map[string]interface{} {
	req, _ := http.NewRequest("GET", fmt.Sprintf("%s/knowledge-bases/%d", baseURL, kbID), nil)
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed to get knowledge base: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Expected status 200, got %d. Body: %s", resp.StatusCode, body)
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	data, ok := result["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("Invalid response format")
	}

	kb, ok := data["knowledge_base"].(map[string]interface{})
	if !ok {
		t.Fatalf("No knowledge base in response")
	}

	return kb
}