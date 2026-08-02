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
