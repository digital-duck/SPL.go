
recipe suggested by Qwen 3.6 Plus

```bash
spl-go run cookbook/49_regulatory_news_audit/audit_news.spl \
  --adapter ollama --model gemma3 \
  news_batch_path="cookbook/49_regulatory_news_audit/data/news_feed.txt" \
  --tools cookbook/tools/finance_helpers.py
```

