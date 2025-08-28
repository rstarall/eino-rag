package document

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/ledongthuc/pdf"
	"go.uber.org/zap"
	"golang.org/x/net/html"
)

type DocumentParser struct {
	logger *zap.Logger
}

func NewDocumentParser(logger *zap.Logger) *DocumentParser {
	return &DocumentParser{
		logger: logger,
	}
}

// ParseDocument 解析文档内容
func (p *DocumentParser) ParseDocument(filename string, content []byte) (string, error) {
	ext := strings.ToLower(filepath.Ext(filename))
	
	switch ext {
	case ".txt", ".md", ".markdown":
		return string(content), nil
	case ".pdf":
		return p.parsePDF(content)
	case ".json":
		return p.parseJSON(content)
	case ".csv":
		return p.parseCSV(content)
	case ".html", ".htm":
		return p.parseHTML(content)
	default:
		return "", fmt.Errorf("unsupported file type: %s", ext)
	}
}

// parsePDF 解析PDF文件
func (p *DocumentParser) parsePDF(content []byte) (string, error) {
	reader := bytes.NewReader(content)
	pdfReader, err := pdf.NewReader(reader, int64(len(content)))
	if err != nil {
		return "", fmt.Errorf("failed to create PDF reader: %w", err)
	}

	var text strings.Builder
	numPages := pdfReader.NumPage()
	
	p.logger.Info("Starting PDF parsing",
		zap.Int("total_pages", numPages),
		zap.Int("content_size", len(content)))
	
	for i := 1; i <= numPages; i++ {
		// 记录解析进度
		if i%10 == 0 || i == numPages {
			p.logger.Info("PDF parsing progress",
				zap.Int("current_page", i),
				zap.Int("total_pages", numPages),
				zap.Float64("progress", float64(i)/float64(numPages)*100))
		}
		
		page := pdfReader.Page(i)
		if page.V.IsNull() {
			continue
		}
		
		pageText, err := page.GetPlainText(nil)
		if err != nil {
			p.logger.Warn("Failed to extract text from PDF page",
				zap.Int("page", i),
				zap.Error(err))
			continue
		}
		
		text.WriteString(pageText)
		text.WriteString("\n\n")
	}

	result := strings.TrimSpace(text.String())
	if result == "" {
		return "", fmt.Errorf("no text content found in PDF")
	}

	return result, nil
}

// parseJSON 解析JSON文件
func (p *DocumentParser) parseJSON(content []byte) (string, error) {
	var data interface{}
	if err := json.Unmarshal(content, &data); err != nil {
		return "", fmt.Errorf("failed to parse JSON: %w", err)
	}

	// 美化输出JSON
	formatted, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to format JSON: %w", err)
	}

	return string(formatted), nil
}

// parseCSV 解析CSV文件
func (p *DocumentParser) parseCSV(content []byte) (string, error) {
	reader := csv.NewReader(bytes.NewReader(content))
	
	var result strings.Builder
	records, err := reader.ReadAll()
	if err != nil {
		return "", fmt.Errorf("failed to parse CSV: %w", err)
	}

	for i, record := range records {
		result.WriteString(strings.Join(record, " | "))
		result.WriteString("\n")
		
		// 在标题行后添加分隔线
		if i == 0 && len(records) > 1 {
			result.WriteString(strings.Repeat("-", 50))
			result.WriteString("\n")
		}
	}

	return result.String(), nil
}

// parseHTML 解析HTML文件
func (p *DocumentParser) parseHTML(content []byte) (string, error) {
	doc, err := html.Parse(bytes.NewReader(content))
	if err != nil {
		return "", fmt.Errorf("failed to parse HTML: %w", err)
	}

	var text strings.Builder
	var extractText func(*html.Node)
	
	extractText = func(n *html.Node) {
		if n.Type == html.TextNode {
			text.WriteString(strings.TrimSpace(n.Data))
			text.WriteString(" ")
		}
		
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			extractText(c)
		}
	}

	extractText(doc)
	
	// 清理多余的空格
	result := strings.Join(strings.Fields(text.String()), " ")
	return result, nil
}

// ValidateFileType 验证文件类型是否支持
func (p *DocumentParser) ValidateFileType(filename string, allowedTypes []string) error {
	ext := strings.ToLower(filepath.Ext(filename))
	
	// Debug logging
	p.logger.Debug("Validating file type",
		zap.String("filename", filename),
		zap.String("ext", ext),
		zap.Strings("allowed_types", allowedTypes))
	
	for _, allowed := range allowedTypes {
		if ext == allowed {
			return nil
		}
	}
	
	return fmt.Errorf("file type %s is not allowed", ext)
}