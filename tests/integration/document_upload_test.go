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

const (
	baseURL = "http://localhost:8080/api/v1"
	testPDF = `%PDF-1.4
1 0 obj
<< /Type /Catalog /Pages 2 0 R >>
endobj
2 0 obj
<< /Type /Pages /Kids [3 0 R] /Count 1 >>
endobj
3 0 obj
<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Resources << /Font << /F1 << /Type /Font /Subtype /Type1 /BaseFont /Helvetica >> >> >> /Contents 4 0 R >>
endobj
4 0 obj
<< /Length 44 >>
stream
BT
/F1 12 Tf
100 100 Td
(Test PDF) Tj
ET
endstream
endobj
xref
0 5
0000000000 65535 f
0000000009 00000 n
0000000058 00000 n
0000000115 00000 n
0000000308 00000 n
trailer
<< /Size 5 /Root 1 0 R >>
startxref
402
%%EOF`
)

func TestDocumentUpload_DuplicateHandling(t *testing.T) {
	// 首先获取token
	token := loginAndGetToken(t)

	// 创建知识库
	kbID := createKnowledgeBase(t, token)

	// 第一次上传文档
	docID1 := uploadDocument(t, token, kbID, "test.pdf", []byte(testPDF))
	t.Logf("First upload successful, document ID: %d", docID1)

	// 第二次上传相同的文档（相同内容）
	uploadDuplicateDocument(t, token, kbID, "test_duplicate.pdf", []byte(testPDF))
}

func loginAndGetToken(t *testing.T) string {
	payload := map[string]string{
		"username": "admin",
		"password": "admin123",
	}

	body, _ := json.Marshal(payload)
	resp, err := http.Post(baseURL+"/auth/login", "application/json", bytes.NewBuffer(body))
	if err != nil {
		t.Fatalf("Failed to login: %v", err)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	data, ok := result["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("Invalid response format")
	}

	token, ok := data["token"].(string)
	if !ok {
		t.Fatalf("No token in response")
	}

	return token
}

func createKnowledgeBase(t *testing.T, token string) uint {
	payload := map[string]string{
		"name":        fmt.Sprintf("Test KB %d", time.Now().Unix()),
		"description": "Test knowledge base for integration test",
	}

	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", baseURL+"/knowledge-bases", bytes.NewBuffer(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed to create knowledge base: %v", err)
	}
	defer resp.Body.Close()

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

	id, ok := kb["id"].(float64)
	if !ok {
		t.Fatalf("No ID in knowledge base")
	}

	return uint(id)
}

func uploadDocument(t *testing.T, token string, kbID uint, filename string, content []byte) uint {
	var b bytes.Buffer
	w := multipart.NewWriter(&b)

	fw, err := w.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("Failed to create form file: %v", err)
	}

	if _, err := io.Copy(fw, bytes.NewReader(content)); err != nil {
		t.Fatalf("Failed to copy file content: %v", err)
	}

	w.Close()

	req, _ := http.NewRequest("POST", fmt.Sprintf("%s/knowledge-bases/%d/documents", baseURL, kbID), &b)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", w.FormDataContentType())

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed to upload document: %v", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	t.Logf("Upload response: %s", string(respBody))

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("Expected status 201, got %d", resp.StatusCode)
	}

	var result map[string]interface{}
	json.Unmarshal(respBody, &result)

	data, _ := result["data"].(map[string]interface{})
	doc, _ := data["document"].(map[string]interface{})
	id, _ := doc["id"].(float64)

	return uint(id)
}

func uploadDuplicateDocument(t *testing.T, token string, kbID uint, filename string, content []byte) {
	var b bytes.Buffer
	w := multipart.NewWriter(&b)

	fw, err := w.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("Failed to create form file: %v", err)
	}

	if _, err := io.Copy(fw, bytes.NewReader(content)); err != nil {
		t.Fatalf("Failed to copy file content: %v", err)
	}

	w.Close()

	req, _ := http.NewRequest("POST", fmt.Sprintf("%s/knowledge-bases/%d/documents", baseURL, kbID), &b)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", w.FormDataContentType())

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed to upload document: %v", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	t.Logf("Duplicate upload response: %s", string(respBody))

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("Expected status 400 for duplicate document, got %d", resp.StatusCode)
	}

	var result map[string]interface{}
	json.Unmarshal(respBody, &result)

	errorMsg, _ := result["error"].(string)
	if errorMsg == "" || !bytes.Contains([]byte(errorMsg), []byte("already exists")) {
		t.Fatalf("Expected error about document already exists, got: %s", errorMsg)
	}

	t.Log("Duplicate document correctly rejected")
}