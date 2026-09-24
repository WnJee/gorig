# Repository & AI Guidelines

## Project Structure & Module Organization
- `apix/` Generic HTTP request binding (`BindReq`), parameter extraction, standard responses (`Ok`, `OkPage`, `Err`), and panic recovery.
- `httpx/` HTTP engine, security middlewares, generic HTTP client (`GetJSON`, `Post`), and SSE streaming (`ssex`).
- `domainx/` (`dx`) Fluent type-safe ORM across MySQL, SQLite, and MongoDB; transactions; Snowflake ID.
- `cache/` Multi-level caching (Memory, Redis, SQLite, JSON), anti-stampede Singleflight (`Remember`), distributed locking (`WithLock`, `WithLockTimeout`).
- `cronx/` Cron expressions, fixed intervals, delay tasks, and Redis distributed task leasing.
- `mid/tokenx/` JWT authentication, custom claims parsing, token revocation/blacklisting.
- `mid/messagex/` Pub/Sub event bus for local memory and Redis event streaming (`PublishEvent`, `SubscribeEvent`).
- `storage/` Unified object storage interface supporting Local disk, AWS S3, Aliyun OSS, and MinIO.
- `utils/` Structured logging (`logger`), layered config (`cofigure`), crypto (`encrypt`), error models (`errors`), and multi-channel alerting (`alert`).
- `bootstrap/` Application startup lifecycle wiring.
- `serv/` Service entry points; `simple/` runnable example.
- `skills/gorig-agent/` AI Agent Skill definition and comprehensive implementation standard.

## AI Agent Skill
- The repository provides the official `gorig-agent` skill in `skills/gorig-agent/SKILL.md`.
- AI agents should reference this skill when scaffolding new modules or implementing business logic.

## Build, Verification, and Development Commands
- Run example: `go run ./simple`
- Build all: `go build ./...`
- Lint basics: `go vet ./...`
- Format: `go fmt ./...` (enforce before commits)

## Coding Style & Naming Conventions
- Follow standard Go conventions (`gofmt`); tabs; clean and concise modern Go with generics.
- Strict 4-tier layer boundary: `Router` -> `Controller` -> `Service` -> `Model / DX`.
- Controllers must use `apix.BindReq[ReqDTO](c)` and return via `apix.Ok` / `apix.Err`.
- Avoid historical fallback shims or legacy dirty data branches; keep code modern and idiomatic.
- Universal utilities and conversions must be extracted and reused, avoiding copy-pasted code.

## Testing & Verification Policy
- Daily validation must be performed using `go vet ./...` and `go build ./...`.
- Unless explicitly requested by the user, **do NOT persist new `*_test.go` files** in the repository. Temporary test verification files must be removed immediately after validation to keep the workspace clean.

## Commit Guidelines
- Commit messages follow Conventional Commits: `feat:`, `fix:`, `refactor:`, `docs:`, `chore:`.

