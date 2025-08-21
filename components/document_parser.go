package components

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	fastpdf "github.com/ledongthuc/pdf"
	"go.uber.org/zap"
)

// DocumentParser 处理不同类型的文档
type DocumentParser struct {
	logger *zap.Logger
}

func NewDocumentParser(logger *zap.Logger) *DocumentParser {
	logger.Info("初始化文档解析器，使用 ledongthuc/pdf 库进行PDF解析")

	return &DocumentParser{
		logger: logger,
	}
}

// ParseDocument 根据文件类型解析文档内容
func (p *DocumentParser) ParseDocument(filename string, content []byte) (string, error) {
	ext := strings.ToLower(filepath.Ext(filename))

	p.logger.Info("开始解析文档",
		zap.String("filename", filename),
		zap.String("extension", ext),
		zap.Int("content_size", len(content)))

	switch ext {
	case ".txt", ".md", ".markdown":
		return p.parseTextFile(content)
	case ".json":
		return p.parseJSONFile(content)
	case ".csv":
		return p.parseCSVFile(content)
	case ".html", ".htm":
		return p.parseHTMLFile(content)
	case ".pdf":
		return p.parsePDFFile(filename, content)
	default:
		// 对于未知类型，尝试作为文本处理
		p.logger.Warn("未知文件类型，尝试作为文本处理", zap.String("extension", ext))
		return p.parseTextFile(content)
	}
}

// parseTextFile 解析纯文本文件（txt, md, markdown）
func (p *DocumentParser) parseTextFile(content []byte) (string, error) {
	text := string(content)

	// 清理文本
	text = p.cleanText(text)

	p.logger.Debug("文本文件解析完成",
		zap.Int("original_length", len(content)),
		zap.Int("cleaned_length", len(text)))

	return text, nil
}

// parseJSONFile 解析JSON文件
func (p *DocumentParser) parseJSONFile(content []byte) (string, error) {
	// 简单处理：将JSON作为文本返回
	// 可以根据需要进行更复杂的JSON解析
	text := string(content)
	text = p.cleanText(text)

	p.logger.Debug("JSON文件解析完成", zap.Int("length", len(text)))
	return text, nil
}

// parseCSVFile 解析CSV文件
func (p *DocumentParser) parseCSVFile(content []byte) (string, error) {
	// 简单处理：将CSV作为文本返回
	// 可以根据需要进行更复杂的CSV解析
	text := string(content)
	text = p.cleanText(text)

	p.logger.Debug("CSV文件解析完成", zap.Int("length", len(text)))
	return text, nil
}

// parseHTMLFile 解析HTML文件
func (p *DocumentParser) parseHTMLFile(content []byte) (string, error) {
	text := string(content)

	// 简单的HTML标签清理（可以使用更sophisticated的HTML parser）
	text = p.removeHTMLTags(text)
	text = p.cleanText(text)

	p.logger.Debug("HTML文件解析完成",
		zap.Int("original_length", len(content)),
		zap.Int("cleaned_length", len(text)))

	return text, nil
}

// parsePDFFile 使用ledongthuc/pdf库解析PDF文件
func (p *DocumentParser) parsePDFFile(filename string, content []byte) (string, error) {
	startTime := time.Now()
	p.logger.Info("开始PDF解析",
		zap.String("filename", filename),
		zap.Int("file_size", len(content)))

	// 创建临时文件来存储PDF数据，因为ledongthuc/pdf库只支持文件路径
	tempFile, err := ioutil.TempFile("", "temp_pdf_*.pdf")
	if err != nil {
		p.logger.Error("PDF解析失败 - 无法创建临时文件",
			zap.String("filename", filename),
			zap.Error(err),
			zap.Duration("duration", time.Since(startTime)))
		return "", fmt.Errorf("无法创建临时文件: %w", err)
	}
	defer os.Remove(tempFile.Name()) // 确保删除临时文件
	defer tempFile.Close()

	// 将PDF数据写入临时文件
	if _, err := tempFile.Write(content); err != nil {
		p.logger.Error("PDF解析失败 - 无法写入临时文件",
			zap.String("filename", filename),
			zap.Error(err),
			zap.Duration("duration", time.Since(startTime)))
		return "", fmt.Errorf("无法写入临时文件: %w", err)
	}

	// 关闭文件以确保数据被刷新到磁盘
	tempFile.Close()

	// 打开PDF文件
	f, r, err := fastpdf.Open(tempFile.Name())
	if err != nil {
		p.logger.Error("PDF解析失败 - 无法打开PDF",
			zap.String("filename", filename),
			zap.String("temp_file", tempFile.Name()),
			zap.Error(err),
			zap.Duration("duration", time.Since(startTime)))
		return "", fmt.Errorf("无法打开PDF文件: %w", err)
	}
	defer f.Close()

	// 获取PDF文档的纯文本内容
	textReader, err := r.GetPlainText()
	if err != nil {
		p.logger.Error("PDF解析失败 - 无法提取文本",
			zap.String("filename", filename),
			zap.Error(err),
			zap.Duration("duration", time.Since(startTime)))
		return "", fmt.Errorf("无法提取PDF文本: %w", err)
	}

	var buffer bytes.Buffer
	_, err = buffer.ReadFrom(textReader)
	if err != nil {
		p.logger.Error("PDF解析失败 - 无法读取文本流",
			zap.String("filename", filename),
			zap.Error(err),
			zap.Duration("duration", time.Since(startTime)))
		return "", fmt.Errorf("无法读取PDF文本流: %w", err)
	}

	text := buffer.String()
	text = p.cleanText(text)

	// 检查解析结果
	if len(strings.TrimSpace(text)) == 0 {
		p.logger.Warn("PDF解析成功但内容为空", zap.String("filename", filename))
		return "", errors.New("PDF文件解析成功，但未提取到文本内容，可能是扫描版PDF或图片PDF")
	}

	duration := time.Since(startTime)
	p.logger.Info("PDF解析完成",
		zap.String("filename", filename),
		zap.Int("original_size", len(content)),
		zap.Int("extracted_length", len(text)),
		zap.Int("total_pages", r.NumPage()),
		zap.Duration("duration", duration))

	return text, nil
}

// cleanText 清理文本内容
func (p *DocumentParser) cleanText(text string) string {
	// 替换多个连续的换行符为单个换行符
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")

	// 移除多余的空白字符
	lines := strings.Split(text, "\n")
	var cleanedLines []string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			cleanedLines = append(cleanedLines, line)
		}
	}

	return strings.Join(cleanedLines, "\n")
}

// removeHTMLTags 简单的HTML标签移除
func (p *DocumentParser) removeHTMLTags(text string) string {
	// 这是一个简单的实现，实际项目中建议使用专门的HTML解析库
	var result strings.Builder
	inTag := false

	for _, char := range text {
		switch char {
		case '<':
			inTag = true
		case '>':
			inTag = false
		default:
			if !inTag {
				result.WriteRune(char)
			}
		}
	}

	return result.String()
}

// GetSupportedExtensions 返回支持的文件扩展名
func (p *DocumentParser) GetSupportedExtensions() []string {
	return []string{".txt", ".md", ".markdown", ".json", ".csv", ".html", ".htm", ".pdf"}
}

// IsSupported 检查文件类型是否支持
func (p *DocumentParser) IsSupported(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	supported := p.GetSupportedExtensions()

	for _, supportedExt := range supported {
		if ext == supportedExt {
			return true
		}
	}

	return false
}
