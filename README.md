# Telepharmacy Tasks API

Go Fiber CRUD API backed by Supabase PostgreSQL. The API never modifies the seed file.

## Setup

1. Create `.env` from `.env.example`.
2. Set `DATABASE_URL` to the Supabase PostgreSQL connection string. The project host is already configured in `.env`; replace `REPLACE_WITH_DATABASE_PASSWORD` with the database password from Supabase Dashboard > Connect. For a long-running backend, use the direct connection or session pooler and keep `sslmode=require`.
3. Set `SEED_FILE_PATH=/Users/thawatchai/Downloads/db.json`.
4. Set `CORS_ORIGINS` to the Frontend origin, for example `https://frontend.example.com`. Multiple origins can be comma-separated.
5. Ensure the Supabase table exists:

```sql
create table if not exists public.tasks (
  id uuid primary key,
  customer_name text,
  service_type text not null,
  symptom text,
  status text not null,
  created_at text not null
);
```

If the table already exists with `created_at timestamptz`, run the SQL migration in `supabase/migrations/20260714000000_align_tasks_seed_schema.sql` first. It also adds `pending` to the allowed statuses.

6. Install dependencies and run:

```bash
go mod tidy
go run .
```

Run with Docker Compose:

```bash
docker compose up --build
```

The API is available at `http://localhost:3000`.

## Routes

```text
GET    /health
GET    /api/v1/tasks
GET    /api/v1/tasks/:id
POST   /api/v1/tasks
PUT    /api/v1/tasks/:id
PATCH  /api/v1/tasks/:id
DELETE /api/v1/tasks/:id
POST   /api/v1/tasks/seed
```

Seed deletes all existing tasks in a transaction, inserts every task from `db.json`, and generates a new UUID for every row.

## Deploy to Google Cloud Run

Build and deploy from the repository root:

```bash
gcloud builds submit --tag REGION-docker.pkg.dev/PROJECT_ID/REPOSITORY/telepharmacy-api
gcloud run deploy telepharmacy-api \
  --image REGION-docker.pkg.dev/PROJECT_ID/REPOSITORY/telepharmacy-api \
  --region REGION \
  --allow-unauthenticated \
  --set-env-vars PORT=8080,SEED_FILE_PATH=/app/db.json \
  --set-env-vars DATABASE_URL='postgresql://...'
```

Cloud Run provides `PORT` automatically and the server listens on it. For production, keep `DATABASE_URL` in Secret Manager instead of passing it directly on the command line. The current repository does not contain `db.json`; copy the unchanged seed file into the repository as `db.json` before building if the Cloud Run seed endpoint is required.
