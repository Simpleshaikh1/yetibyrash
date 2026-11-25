# Yetibyrash

Elegance meets affordability. Online shop for jewelry, watches, bags, women's clothing and intimates in Nigeria.

- `backend/` Go API (one binary: API, background jobs, and later the built React apps)
- `frontend/shop` customer site · `frontend/admin` staff dashboard · `frontend/ui` shared tokens and helpers
- `design/` canvas source and design system · `docs/backend-plan.md` the plan

## First-time setup

Needs Go 1.27+, Node 22+, Docker Desktop (running).

```powershell
cd frontend
npm install
```

## Run everything (GoLand)

Open the `Yetibyrash` folder as the project, then pick **Yetibyrash (all)** in the run configurations and press Run. It starts:

| Config | What | Address |
| --- | --- | --- |
| `db` | Postgres and Mailpit in Docker | Postgres `localhost:5432`, test email inbox http://localhost:8025 |
| `api` | Go API (migrates the database on start) | http://localhost:8090/api/health |
| `shop` | Customer site | http://localhost:5173 |
| `admin` | Staff dashboard | http://localhost:5174/admin/ |

The API waits up to 30 seconds for the database, so starting everything at once is fine.

## Without GoLand

```powershell
docker compose up -d
```

```powershell
cd backend; go run ./cmd/yetibyrash
```

```powershell
cd frontend; npm run dev:shop
```

## Tests

```powershell
cd backend; go test ./...
```

## Settings

The API reads environment variables, with development defaults for all of them. To override locally, copy `backend/.env.example` to `backend/.env`. Port 8080 is avoided because Apache already uses it on this machine.

## Database queries (sqlc)

SQL lives in `backend/queries/`. Generate the Go code with Docker:

```powershell
cd backend; docker run --rm -v "${PWD}:/src" -w /src sqlc/sqlc:1.27.0 generate
```
