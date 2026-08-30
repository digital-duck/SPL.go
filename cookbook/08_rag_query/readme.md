# Recipe 08: RAG Query

Retrieval-augmented generation — indexes documents as paragraph chunks, retrieves the most semantically relevant sections, and answers questions from that context.

## Setup

```bash
pip install "dd-vectordb[faiss]" "dd-embed[sentence-transformers]"
```

The vector store uses `all-MiniLM-L6-v2` (384-dim, CPU) via `dd-embed` for semantic embeddings. The embedding backend is abstracted by `dd-embed` — swap to `ollama`, `openai`, or any other provider by changing `embed_provider` in `get_vector_store()`.

### Reset the vector store

```bash
rm -f .spl/vectors.faiss .spl/vectors.faiss.meta .spl/vector_config.json
```

### Index a document by paragraph (one-time setup)

Chunking by paragraph gives precise retrieval — `top_k=3` returns the 3 most *semantically relevant* sections rather than the whole document.

```bash
spl doc-rag add /home/gongai/projects/digital-duck/zinets/who-is-wen.md
```

To index as a single document instead:

```bash
spl doc-rag add /home/gongai/projects/digital-duck/zinets/who-is-wen.md --no-chunk
```

Verify:

```bash
spl doc-rag count
spl doc-rag query "Who is Wen?" --top-k 3
```

## Parameters

| Parameter | Type | Default | Description |
|---|---|---|---|
| `question` | TEXT | *(required)* | The question to answer using indexed documents |

## Usage

```bash
spl-go run cookbook/08_rag_query/rag_query.spl --adapter ollama \
    question="Who is Wen?"

spl-go run cookbook/08_rag_query/rag_query.spl --adapter ollama \
    question="What is Momagrid and why was it built?"

spl-go run cookbook/08_rag_query/rag_query.spl --adapter ollama \
    question="What is the Gang of Four in the ZiNets research team?"

spl-go run cookbook/08_rag_query/rag_query.spl --adapter ollama \
    question="What is eta_AI and how is it measured?"

spl-go run cookbook/08_rag_query/rag_query.spl --adapter ollama \
    question="How did Wen transition from physics to software engineering?"
```

## Notes

- The embedding model is locked in `.spl/vector_config.json` — the same model is always used for a given index (mixing models corrupts retrieval). Delete the index files to switch models.
- `top_k=3` retrieves the 3 most relevant paragraphs by cosine similarity.
- Run `spl doc-rag query "your question"` to inspect retrieved context before running the full prompt.
- Vector storage: `dd-vectordb` (FAISSVectorDB). Embedding: `dd-embed` (SentenceTransformerAdapter).
- GPU acceleration is not required; CPU inference works well for typical document sizes.
