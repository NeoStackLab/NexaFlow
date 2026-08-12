package service

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/NeoStackLab/NexaFlow/backend/internal/model"
	pdf "github.com/ledongthuc/pdf"
)

var ErrInvalidKnowledgeDocument = errors.New("invalid knowledge document")

type KnowledgeRepository interface {
	Save(context.Context, string, string, string, string, int64, []model.KnowledgeChunkInput) (model.KnowledgeDocument, error)
	List(context.Context, string) ([]model.KnowledgeDocument, error)
	Delete(context.Context, string, string) (int64, error)
	Search(context.Context, string, []float32, int) ([]model.KnowledgeSource, error)
}
type KnowledgeService interface {
	Ingest(context.Context, string, string, string, string, int64, io.Reader) (model.KnowledgeDocument, error)
	List(context.Context, string) ([]model.KnowledgeDocument, error)
	Delete(context.Context, string, string) error
	Search(context.Context, string, string, int) ([]model.KnowledgeSource, error)
}
type knowledgeService struct {
	repository KnowledgeRepository
	provider   AIProvider
	meter      UsageMeter
}

func NewKnowledgeService(repository KnowledgeRepository, provider AIProvider, meters ...UsageMeter) KnowledgeService {
	var meter UsageMeter
	if len(meters) > 0 {
		meter = meters[0]
	}
	return &knowledgeService{repository: repository, provider: provider, meter: meter}
}
func (s *knowledgeService) Ingest(ctx context.Context, tenantID, actorID, name, contentType string, size int64, reader io.Reader) (document model.KnowledgeDocument, err error) {
	name = filepath.Base(strings.TrimSpace(name))
	if name == "." || name == "" || len(name) > 255 {
		return model.KnowledgeDocument{}, fmt.Errorf("%w: invalid file name", ErrInvalidKnowledgeDocument)
	}
	if size < 1 || size > 20<<20 {
		return model.KnowledgeDocument{}, fmt.Errorf("%w: file must be 1 byte to 20 MiB", ErrInvalidKnowledgeDocument)
	}
	if s.meter != nil {
		if err = s.meter.Consume(ctx, tenantID, "knowledge_bytes", size); err != nil {
			return model.KnowledgeDocument{}, err
		}
		defer func() {
			if err != nil {
				_ = s.meter.Consume(context.WithoutCancel(ctx), tenantID, "knowledge_bytes", -size)
			}
		}()
	}
	data, err := io.ReadAll(io.LimitReader(reader, (20<<20)+1))
	if err != nil {
		return model.KnowledgeDocument{}, err
	}
	text, err := extractKnowledgeText(name, data)
	if err != nil {
		return model.KnowledgeDocument{}, err
	}
	chunks := splitKnowledge(text, 1200, 180)
	if len(chunks) == 0 || len(chunks) > 1000 {
		return model.KnowledgeDocument{}, fmt.Errorf("%w: document produced no usable chunks or exceeds 1000 chunks", ErrInvalidKnowledgeDocument)
	}
	embeddings, err := s.provider.Embed(ctx, chunks)
	if err != nil {
		return model.KnowledgeDocument{}, err
	}
	inputs := make([]model.KnowledgeChunkInput, len(chunks))
	for index := range chunks {
		inputs[index] = model.KnowledgeChunkInput{Position: index, Content: chunks[index], Embedding: embeddings[index]}
	}
	document, err = s.repository.Save(ctx, tenantID, actorID, name, contentType, size, inputs)
	return document, err
}
func (s *knowledgeService) List(ctx context.Context, tenantID string) ([]model.KnowledgeDocument, error) {
	return s.repository.List(ctx, tenantID)
}
func (s *knowledgeService) Delete(ctx context.Context, tenantID, id string) error {
	size, err := s.repository.Delete(ctx, tenantID, id)
	if err != nil {
		return err
	}
	if s.meter != nil {
		return s.meter.Consume(context.WithoutCancel(ctx), tenantID, "knowledge_bytes", -size)
	}
	return nil
}
func (s *knowledgeService) Search(ctx context.Context, tenantID, query string, limit int) ([]model.KnowledgeSource, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("%w: search query is required", ErrInvalidKnowledgeDocument)
	}
	if limit < 1 || limit > 20 {
		limit = 5
	}
	embeddings, err := s.provider.Embed(ctx, []string{query})
	if err != nil {
		return nil, err
	}
	return s.repository.Search(ctx, tenantID, embeddings[0], limit)
}

func extractKnowledgeText(name string, data []byte) (string, error) {
	ext := strings.ToLower(filepath.Ext(name))
	var text string
	var err error
	switch ext {
	case ".txt", ".md", ".csv", ".json":
		if !utf8.Valid(data) {
			return "", fmt.Errorf("%w: text file is not UTF-8", ErrInvalidKnowledgeDocument)
		}
		text = string(data)
	case ".pdf":
		reader, err := pdf.NewReader(bytes.NewReader(data), int64(len(data)))
		if err != nil {
			return "", fmt.Errorf("%w: parse PDF: %v", ErrInvalidKnowledgeDocument, err)
		}
		var builder strings.Builder
		pages := reader.NumPage()
		for page := 1; page <= pages; page++ {
			content := reader.Page(page)
			if content.V.IsNull() {
				continue
			}
			plain, err := content.GetPlainText(nil)
			if err != nil {
				return "", fmt.Errorf("%w: extract PDF text: %v", ErrInvalidKnowledgeDocument, err)
			}
			builder.WriteString(plain)
			builder.WriteByte('\n')
		}
		text = builder.String()
	case ".docx", ".xlsx":
		text, err = extractOfficeXML(data, ext)
		if err != nil {
			return "", err
		}
	default:
		return "", fmt.Errorf("%w: supported files are PDF, DOCX, XLSX, TXT, MD, CSV, and JSON", ErrInvalidKnowledgeDocument)
	}
	text = normalizeText(text)
	if len([]rune(text)) < 20 {
		return "", fmt.Errorf("%w: extracted text is too short", ErrInvalidKnowledgeDocument)
	}
	return text, nil
}
func extractOfficeXML(data []byte, ext string) (string, error) {
	archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("%w: invalid Office document", ErrInvalidKnowledgeDocument)
	}
	targets := []string{"word/document.xml"}
	if ext == ".xlsx" {
		targets = []string{"xl/sharedStrings.xml"}
		for _, file := range archive.File {
			if strings.HasPrefix(file.Name, "xl/worksheets/sheet") && strings.HasSuffix(file.Name, ".xml") {
				targets = append(targets, file.Name)
			}
		}
	}
	wanted := map[string]bool{}
	for _, target := range targets {
		wanted[target] = true
	}
	var builder strings.Builder
	const maximumExtractedBytes = 50 << 20
	extractedBytes := int64(0)
	for _, file := range archive.File {
		if !wanted[file.Name] {
			continue
		}
		if file.UncompressedSize64 > maximumExtractedBytes || extractedBytes+int64(file.UncompressedSize64) > maximumExtractedBytes {
			return "", fmt.Errorf("%w: Office document expands beyond 50 MiB", ErrInvalidKnowledgeDocument)
		}
		extractedBytes += int64(file.UncompressedSize64)
		source, err := file.Open()
		if err != nil {
			return "", err
		}
		decoder := xml.NewDecoder(source)
		for {
			token, err := decoder.Token()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				source.Close()
				return "", fmt.Errorf("%w: invalid Office XML", ErrInvalidKnowledgeDocument)
			}
			if chars, ok := token.(xml.CharData); ok {
				builder.Write(chars)
				builder.WriteByte(' ')
			}
		}
		source.Close()
	}
	return builder.String(), nil
}

var whitespace = regexp.MustCompile(`\s+`)

func normalizeText(text string) string {
	return strings.TrimSpace(whitespace.ReplaceAllString(text, " "))
}
func splitKnowledge(text string, size, overlap int) []string {
	runes := []rune(text)
	chunks := []string{}
	for start := 0; start < len(runes); {
		end := min(start+size, len(runes))
		chunk := strings.TrimSpace(string(runes[start:end]))
		if chunk != "" {
			chunks = append(chunks, chunk)
		}
		if end == len(runes) {
			break
		}
		start = end - overlap
	}
	return chunks
}
