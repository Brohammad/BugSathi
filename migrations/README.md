# Migrations

Apply with:

```bash
make up        # Postgres must be running
make migrate
```

| File | Milestone |
|------|-----------|
| `0001_auth.sql` | M3 — users + refresh_tokens |
| `0002_projects.sql` | M4 — projects + project_members |
| `0003_recordings.sql` | M5 — recordings + outbox |
| `0004_media_artifacts.sql` | M6 — media_artifacts |
| `0005_ai_reports.sql` | M7 — analyses + reports |
| `0006_share_links.sql` | M9 — share_links |
| `0007_report_comments.sql` | M10 — report_comments |
| `0008_media_processing_claim.sql` | M17 — media processing claim |
| `0009_share_token_hash.sql` | M22 — hashed share tokens |
| `0010_kafka_retry_attempts.sql` | M28 — durable Kafka retry counters |
