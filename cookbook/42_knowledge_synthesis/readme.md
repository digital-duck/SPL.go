# Recipe 42: Knowledge Synthesis (Knowledge Evolution)

## Description
This recipe demonstrates how agents can contribute back to a knowledge base, moving beyond "read-only" RAG. It shows a pattern where the model synthesizes new raw information and then uses a deterministic `CALL rag_update()` to commit those insights to a vector store or long-term memory.

## Key Features
- **Knowledge Evolution:** Demonstrates the feedback loop of learning and updating internal knowledge.
- **RAG-Writer Pattern:** Shows how an agent can be a producer of structured knowledge, not just a consumer.
- **Integration with Vector Stores:** Uses a `CALL` statement to bridge the probabilistic model output and deterministic storage.

## Usage
```bash
spl-go run cookbook-v3.0/42_knowledge_synthesis/knowledge_synthesis.spl --tools cookbook-v3.0/tools.py
```
