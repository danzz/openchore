# Database Configuration & PostgreSQL Support

OpenChore supports dual database engines:
- **SQLite** (default): Embedded, file-based (`openchore.db`), ideal for single-instance, homelab, and edge setups.
- **PostgreSQL**: High-performance client-server database, ideal for production, clustering, multi-tenant deployments, and shared NAS/database servers.

---

## 1. Automatic Engine Selection

OpenChore automatically chooses the engine based on your environment:
- If `POSTGRES_HOST` or `DATABASE_URL` is set, OpenChore automatically connects to PostgreSQL.
- You can also explicitly specify `DB_TYPE=postgres` or `DB_TYPE=sqlite`.
- If no PostgreSQL settings are present, it seamlessly defaults to SQLite (`openchore.db`).

---

## 2. PostgreSQL Configuration

Configure PostgreSQL via `.env` or system environment variables:

```bash
# PostgreSQL Server Details
DB_TYPE=postgres
POSTGRES_HOST=192.168.199.11
POSTGRES_PORT=5432
POSTGRES_USER=admin
POSTGRES_PASSWORD=asb@1234
POSTGRES_DATABASE=openchore
POSTGRES_SSLMODE=disable
```

Or using standard `DATABASE_URL`:

```bash
DATABASE_URL=postgres://admin:asb%401234@192.168.199.11:5432/openchore?sslmode=disable
```

### Auto-Creation of Target Database
If the target database (`openchore`) does not exist on the PostgreSQL server, OpenChore will automatically connect to the `postgres` maintenance database on startup and execute `CREATE DATABASE openchore`.

---

## 3. Data Migration (SQLite → PostgreSQL)

If you have existing data in `openchore.db` and wish to transfer everything to PostgreSQL, run:

```bash
make db-migrate-to-postgres
# or
go run cmd/migrate-db/main.go
```

This will:
1. Ensure the PostgreSQL schema and migrations are up to date.
2. Transfer accounts, households, users, chores, schedules, completions, points, and rewards in dependency order.
3. Automatically reset PostgreSQL auto-increment sequences (`SERIAL`) to continue from the latest IDs.

---

## 4. Docker Compose Deployment

Update your `.env` file in the project root:

```bash
POSTGRES_HOST=192.168.199.11
POSTGRES_PORT=5432
POSTGRES_USER=admin
POSTGRES_PASSWORD=asb@1234
POSTGRES_DATABASE=openchore
POSTGRES_SSLMODE=disable
```

Then start the services:

```bash
docker compose up -d
```
