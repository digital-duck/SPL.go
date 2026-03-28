// Package rag provides ChromaDB + Ollama RAG (Retrieval-Augmented Generation) support.
package rag

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// ChromaClient communicates with a running ChromaDB server over HTTP REST.
type ChromaClient struct {
	BaseURL    string // ChromaDB server URL, e.g. "http://localhost:8000"
	OllamaURL  string // Ollama server URL, e.g. "http://localhost:11434"
	EmbedModel string // Ollama embedding model, e.g. "nomic-embed-text"
	Collection string // ChromaDB collection name
	httpClient *http.Client
}

// NewChromaClient creates a new ChromaClient.
func NewChromaClient(baseURL, ollamaURL, embedModel, collection string) *ChromaClient {
	return &ChromaClient{
		BaseURL:    baseURL,
		OllamaURL:  ollamaURL,
		EmbedModel: embedModel,
		Collection: collection,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// chromaError is returned when the ChromaDB server is not reachable.
func (c *ChromaClient) unreachableError() error {
	return fmt.Errorf("chromadb server at %s not reachable — start with: docker run -p 8000:8000 chromadb/chroma", c.BaseURL)
}

// doJSON sends a JSON request and decodes the JSON response into dst.
func (c *ChromaClient) doJSON(ctx context.Context, method, url string, body interface{}, dst interface{}) error {
	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("chroma: marshal request: %w", err)
		}
		reqBody = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return fmt.Errorf("chroma: create request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return c.unreachableError()
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("chroma: read response: %w", err)
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("chroma: HTTP %d: %s", resp.StatusCode, string(data))
	}
	if dst != nil {
		if err := json.Unmarshal(data, dst); err != nil {
			return fmt.Errorf("chroma: unmarshal response: %w (body: %s)", err, string(data))
		}
	}
	return nil
}

// getCollectionID retrieves the collection UUID from ChromaDB.
func (c *ChromaClient) getCollectionID(ctx context.Context) (string, error) {
	url := fmt.Sprintf("%s/api/v1/collections/%s", c.BaseURL, c.Collection)
	var result map[string]interface{}
	if err := c.doJSON(ctx, http.MethodGet, url, nil, &result); err != nil {
		return "", err
	}
	id, ok := result["id"].(string)
	if !ok {
		return "", fmt.Errorf("chroma: collection response missing 'id' field")
	}
	return id, nil
}

// CreateCollection ensures the collection exists (idempotent).
func (c *ChromaClient) CreateCollection(ctx context.Context) error {
	url := fmt.Sprintf("%s/api/v1/collections", c.BaseURL)
	body := map[string]interface{}{
		"name":     c.Collection,
		"metadata": map[string]interface{}{},
	}
	// Ignore 409 Conflict (already exists) — the doJSON will return non-nil on 4xx,
	// so we re-check: just try to get the collection if create fails.
	var result map[string]interface{}
	err := c.doJSON(ctx, http.MethodPost, url, body, &result)
	if err != nil {
		// Collection may already exist; verify by fetching it.
		if _, getErr := c.getCollectionID(ctx); getErr == nil {
			return nil
		}
		return err
	}
	return nil
}

// embed calls Ollama to get an embedding vector for text.
func (c *ChromaClient) embed(ctx context.Context, text string) ([]float64, error) {
	url := fmt.Sprintf("%s/api/embeddings", c.OllamaURL)
	body := map[string]string{
		"model":  c.EmbedModel,
		"prompt": text,
	}
	var result struct {
		Embedding []float64 `json:"embedding"`
	}
	req, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(req))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("ollama embed: server at %s not reachable — run: ollama pull %s", c.OllamaURL, c.EmbedModel)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("ollama embed: read response: %w", err)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("ollama embed: HTTP %d: %s", resp.StatusCode, string(data))
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("ollama embed: unmarshal: %w", err)
	}
	if len(result.Embedding) == 0 {
		return nil, fmt.Errorf("ollama embed: empty embedding returned for model %s", c.EmbedModel)
	}
	return result.Embedding, nil
}

// Add embeds text and stores it in ChromaDB.
func (c *ChromaClient) Add(ctx context.Context, id, text string, metadata map[string]string) error {
	collID, err := c.getCollectionID(ctx)
	if err != nil {
		return err
	}
	embedding, err := c.embed(ctx, text)
	if err != nil {
		return err
	}

	// Convert metadata to map[string]interface{}
	meta := make(map[string]interface{}, len(metadata))
	for k, v := range metadata {
		meta[k] = v
	}

	url := fmt.Sprintf("%s/api/v1/collections/%s/add", c.BaseURL, collID)
	body := map[string]interface{}{
		"ids":        []string{id},
		"embeddings": [][]float64{embedding},
		"documents":  []string{text},
		"metadatas":  []map[string]interface{}{meta},
	}
	return c.doJSON(ctx, http.MethodPost, url, body, nil)
}

// Query embeds text and retrieves the top_k most similar documents.
// Returns a list of maps with keys: id, text, score, and any metadata keys.
func (c *ChromaClient) Query(ctx context.Context, text string, topK int) ([]map[string]string, error) {
	collID, err := c.getCollectionID(ctx)
	if err != nil {
		return nil, err
	}
	embedding, err := c.embed(ctx, text)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/api/v1/collections/%s/query", c.BaseURL, collID)
	body := map[string]interface{}{
		"query_embeddings": [][]float64{embedding},
		"n_results":        topK,
		"include":          []string{"documents", "metadatas", "distances"},
	}

	var result struct {
		IDs       [][]string                   `json:"ids"`
		Documents [][]string                   `json:"documents"`
		Metadatas [][]map[string]interface{}   `json:"metadatas"`
		Distances [][]float64                  `json:"distances"`
	}
	if err := c.doJSON(ctx, http.MethodPost, url, body, &result); err != nil {
		return nil, err
	}

	var out []map[string]string
	if len(result.IDs) == 0 {
		return out, nil
	}
	ids := result.IDs[0]
	docs := result.Documents[0]
	var metas []map[string]interface{}
	if len(result.Metadatas) > 0 {
		metas = result.Metadatas[0]
	}
	var dists []float64
	if len(result.Distances) > 0 {
		dists = result.Distances[0]
	}

	for i, id := range ids {
		entry := map[string]string{"id": id}
		if i < len(docs) {
			entry["text"] = docs[i]
		}
		if i < len(dists) {
			entry["score"] = fmt.Sprintf("%f", dists[i])
		}
		if i < len(metas) {
			for k, v := range metas[i] {
				entry[k] = fmt.Sprintf("%v", v)
			}
		}
		out = append(out, entry)
	}
	return out, nil
}

// Count returns the number of documents in the collection.
func (c *ChromaClient) Count(ctx context.Context) (int, error) {
	collID, err := c.getCollectionID(ctx)
	if err != nil {
		return 0, err
	}
	url := fmt.Sprintf("%s/api/v1/collections/%s/count", c.BaseURL, collID)
	var count int
	if err := c.doJSON(ctx, http.MethodGet, url, nil, &count); err != nil {
		return 0, err
	}
	return count, nil
}

// Delete removes a document by ID.
func (c *ChromaClient) Delete(ctx context.Context, id string) error {
	collID, err := c.getCollectionID(ctx)
	if err != nil {
		return err
	}
	url := fmt.Sprintf("%s/api/v1/collections/%s/delete", c.BaseURL, collID)
	body := map[string]interface{}{
		"ids": []string{id},
	}
	return c.doJSON(ctx, http.MethodPost, url, body, nil)
}
