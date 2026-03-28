package rag

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

const docRAGCollection = "spl_doc_rag"

// DocRAGStore stores and retrieves document chunks via ChromaDB.
type DocRAGStore struct {
	client *ChromaClient
}

// NewDocRAGStore creates a DocRAGStore backed by ChromaDB and Ollama embeddings.
func NewDocRAGStore(chromaURL, ollamaURL, embedModel string) *DocRAGStore {
	return &DocRAGStore{
		client: NewChromaClient(chromaURL, ollamaURL, embedModel, docRAGCollection),
	}
}

// Add splits text into paragraphs (on \n\n), embeds each, and stores them.
// Returns the number of chunks added.
func (d *DocRAGStore) Add(ctx context.Context, text string, metadata map[string]string) (int, error) {
	if err := d.client.CreateCollection(ctx); err != nil {
		return 0, fmt.Errorf("doc-rag: create collection: %w", err)
	}

	// Split on double newlines to get paragraphs
	chunks := splitParagraphs(text)
	count := 0
	for i, chunk := range chunks {
		chunk = strings.TrimSpace(chunk)
		if chunk == "" {
			continue
		}
		id := fmt.Sprintf("%s-chunk-%d", uuid.New().String(), i)
		meta := make(map[string]string, len(metadata)+1)
		for k, v := range metadata {
			meta[k] = v
		}
		meta["chunk_index"] = fmt.Sprintf("%d", i)
		if err := d.client.Add(ctx, id, chunk, meta); err != nil {
			return count, fmt.Errorf("doc-rag: add chunk %d: %w", i, err)
		}
		count++
	}
	return count, nil
}

// Query returns the top_k most similar document chunks.
func (d *DocRAGStore) Query(ctx context.Context, text string, topK int) ([]map[string]string, error) {
	return d.client.Query(ctx, text, topK)
}

// Count returns the total number of indexed chunks.
func (d *DocRAGStore) Count(ctx context.Context) (int, error) {
	return d.client.Count(ctx)
}

// splitParagraphs splits text on blank lines (\n\n).
func splitParagraphs(text string) []string {
	return strings.Split(text, "\n\n")
}
