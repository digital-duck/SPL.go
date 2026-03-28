package rag

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

const codeRAGCollection = "spl_code_rag"

// CodeRAGStore indexes (description, SPL source) pairs for code retrieval.
type CodeRAGStore struct {
	client *ChromaClient
}

// NewCodeRAGStore creates a CodeRAGStore backed by ChromaDB and Ollama embeddings.
func NewCodeRAGStore(chromaURL, ollamaURL, embedModel string) *CodeRAGStore {
	return &CodeRAGStore{
		client: NewChromaClient(chromaURL, ollamaURL, embedModel, codeRAGCollection),
	}
}

// AddPair adds a (description, spl_source) pair to the store.
func (c *CodeRAGStore) AddPair(ctx context.Context, description, splSource string, metadata map[string]string) error {
	if err := c.client.CreateCollection(ctx); err != nil {
		return fmt.Errorf("code-rag: create collection: %w", err)
	}
	id := uuid.New().String()
	meta := make(map[string]string, len(metadata)+1)
	for k, v := range metadata {
		meta[k] = v
	}
	meta["spl_source"] = splSource
	// Embed the description text; spl_source is stored as metadata.
	return c.client.Add(ctx, id, description, meta)
}

// Retrieve finds the top_k most similar (description, spl_source) pairs.
// Returns a list of maps with keys: description, spl_source, score.
func (c *CodeRAGStore) Retrieve(ctx context.Context, description string, topK int) ([]map[string]string, error) {
	results, err := c.client.Query(ctx, description, topK)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]string, 0, len(results))
	for _, r := range results {
		entry := map[string]string{
			"description": r["text"],
			"spl_source":  r["spl_source"],
			"score":       r["score"],
		}
		out = append(out, entry)
	}
	return out, nil
}

// Count returns the number of indexed (description, spl_source) pairs.
func (c *CodeRAGStore) Count(ctx context.Context) (int, error) {
	return c.client.Count(ctx)
}

// catalog is the optional catalog.json structure.
type catalogEntry struct {
	File        string `json:"file"`
	Description string `json:"description"`
}

// IndexCookbook walks cookbookDir for *.spl files, reads each with its
// description (from catalog.json if present, else uses the filename),
// and adds pairs to the store. Returns count of newly added pairs.
func (c *CodeRAGStore) IndexCookbook(ctx context.Context, cookbookDir string) (int, error) {
	if err := c.client.CreateCollection(ctx); err != nil {
		return 0, fmt.Errorf("code-rag: create collection: %w", err)
	}

	// Load catalog.json if present.
	catalog := make(map[string]string) // file -> description
	catalogPath := filepath.Join(cookbookDir, "catalog.json")
	if data, err := os.ReadFile(catalogPath); err == nil {
		var entries []catalogEntry
		if json.Unmarshal(data, &entries) == nil {
			for _, e := range entries {
				catalog[e.File] = e.Description
			}
		}
	}

	// Walk for *.spl files.
	entries, err := os.ReadDir(cookbookDir)
	if err != nil {
		return 0, fmt.Errorf("code-rag: read cookbook dir: %w", err)
	}

	count := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".spl") {
			continue
		}
		splPath := filepath.Join(cookbookDir, entry.Name())
		splData, err := os.ReadFile(splPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "code-rag: skip %s: %v\n", entry.Name(), err)
			continue
		}
		description, ok := catalog[entry.Name()]
		if !ok {
			// Use filename without extension as description.
			description = strings.TrimSuffix(entry.Name(), ".spl")
			description = strings.ReplaceAll(description, "_", " ")
			description = strings.ReplaceAll(description, "-", " ")
		}
		meta := map[string]string{
			"filename": entry.Name(),
			"source":   "cookbook",
		}
		if err := c.AddPair(ctx, description, string(splData), meta); err != nil {
			fmt.Fprintf(os.Stderr, "code-rag: index %s: %v\n", entry.Name(), err)
			continue
		}
		count++
	}
	return count, nil
}
