# AGENTS.md

## Project Overview

`cloudpvp-matcher` is the matchmaking middleware for the game battle platform.

## Architecture: Clean Architecture

This project follows **Clean Architecture** with strict dependency rules:

```
infrastructure → interface → usecase → domain
  (外部框架)       (适配层)      (用例层)      (实体层)
```

### Directory-to-Layer Mapping

| Directory | Clean Arch Layer | Contains |
|---|---|---|
| `internal/domain/` | Entities | entities, value objects, domain service interfaces, repository ports |
| `internal/usecase/` | Use Cases | matchmaking orchestration, ticket lifecycle, state transitions |
| `internal/interface/` | Interface Adapters | message handlers, DTOs, repository implementations |
| `internal/infrastructure/` | Frameworks & Drivers | RabbitMQ conn/publish/consume, Apollo client, Redis client |

### Dependency Rule

- `domain/` MUST NOT import any other `internal/` package.
- `usecase/` MAY import only `domain/`.
- `interface/` MAY import `usecase/` and `domain/`.
- `infrastructure/` MAY import any package (provides concrete implementations).
- `cmd/main.go` is the composition root — the only place where all layers are wired together.

## Responsibility Boundaries

Responsibility boundaries are defined by logic ownership, not by fixed directory structure:

- Domain logic: entities, value objects, matchmaking rules, domain services; no middleware client dependency.
- Use case orchestration: state transitions, idempotency, consistency, and flow orchestration; depend on abstractions, not concrete middleware implementations.
- Interface adaptation: request/response mapping, error mapping, auth context extraction; boundary conversion only.
- Infrastructure implementation: integration details for RabbitMQ and Apollo.

### Adding a New Feature

1. Define domain entities/value objects in `domain/`
2. Define repository/event ports (interfaces) in `domain/repository/`
3. Implement use case in `usecase/<business-capability>/`
4. Implement adapters in `interface/` (handlers, DTOs, repo impls)
5. Wire dependencies in `cmd/matcher/main.go`

### Adding a New Game Mode

1. Add `GameMode` constant in `domain/valueobject/game_mode.go`
2. Implement `Matchmaker` interface in `domain/service/`
3. Register the new matchmaker in `cmd/matcher/main.go`
4. Add match config (in `config.yaml` or Apollo)

## Git Commit Conventions

Use Conventional Commits:

- Format: `<type>(<scope>): <subject>`
- Common types: `feat`, `fix`, `refactor`, `test`, `docs`, `chore`, `perf`, `build`, `ci`
- Recommended `scope`: `queue`, `match-core`, `mmr`, `ticket`
- Commit messages must describe behavior changes precisely; avoid generic text like "fix issue" or "optimize code"

Examples:

- `feat(match-core): add mode-bucket candidate pool building flow`
- `fix(queue): prevent duplicated rematch from timeout ticket double dequeue`
- `refactor(ticket): split ticket validation and enqueue flow with tests`

## Review Guidelines

- First confirm whether a finding has real business impact; avoid re-fixing behavior already protected by validation, idempotency, fallback, or recovery.
- Before changing code, evaluate whether the fix introduces new risk, broadens behavior scope, or over-designs for unlikely scenarios.
- Keep change size proportional to risk and consistent with existing responsibility boundaries and data flow.
- During review, explicitly verify that the dependency boundary is preserved (RabbitMQ and Apollo only).
- After the fix, revisit related call paths to ensure existing flow assumptions still hold.
- After review or fix, add concise local comments when needed to explain why the fix is required or why a risk is intentionally accepted.

## Comment Guidelines

### Code Comments

- Comments must improve readability and maintainability; focus on why, not just what.
- Go comments for functions, methods, and types must be written in Chinese.
- Every Go function and method should have a concise Chinese comment above it, including unexported helpers.
- Add comments for non-obvious constraints, ordering dependencies, tradeoffs, or external API quirks.
- For long flows, add short comments before major blocks to improve scanability.
- Avoid comments that only restate obvious code.
- During review, follow the comment requirements in Review Guidelines.

### New Types / Functions

- Add GoDoc comments for new exported types, functions, and methods.
- For complex unexported functions with non-obvious behavior, add concise intent/constraint comments.
- Prefer Chinese comments for consistency in this repository.

## Testing Baseline

- Business logic changes must include corresponding unit tests.
- Changes involving matchmaking timing, concurrency, timeout, retry, or idempotency must include scenario tests.
- Run `go test ./...` before commit.
