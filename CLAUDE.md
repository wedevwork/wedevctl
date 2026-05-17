# CLAUDE.md — wedevctl

This file provides guidance for Claude Code when working in this repository.

## Project Overview

**wedevctl** is a Go CLI tool for managing WireGuard virtual networks. It handles automatic IP allocation, WireGuard configuration generation, and version-tracked config history using an embedded BoltDB database.

## Build & Run Commands

```bash
# Build
go build -o wedevctl main.go

# Build with size optimizations
go build -ldflags="-s -w" -o wedevctl main.go

# Run tests (all packages)
go test ./...

# Run tests with verbose output and race detector
go test -v -race ./...

# Run tests with coverage
go test -v -race -coverprofile=coverage.out ./...
go tool cover -func=coverage.out

# Run tests for a specific package
go test ./util -v
go test ./wedev -v
go test ./cmd -v

# Run performance benchmarks (not part of `go test`, not a CI gate)
go test -bench=. -benchmem ./util/... ./wedev/...

# Format code — use gofmt -s; CI checks with the -s (simplify) flag,
# which `go fmt` does not apply.
gofmt -s -w .

# Static analysis
go vet ./...
```

## Architecture

```
wedevctl/
├── main.go          # Entry point — creates and executes root Cobra command
├── cmd/
│   ├── root.go      # All CLI command definitions (Cobra); initializes DB, manager, validator
│   └── root_test.go # CLI-level tests
├── wedev/
│   ├── manager.go   # Business logic — VirtualNetworkManager; CRUD for networks, servers, nodes, configs
│   ├── manager_test.go
│   ├── storage.go   # BoltDB persistence — StorageManager; low-level bucket ops
│   └── storage_test.go
├── util/
│   ├── util.go      # IP pool management, input validation (names, CIDR, endpoints, ports)
│   └── util_test.go
├── go.mod
└── design/          # Local symlink to private design docs — NOT committed;
                     # absent for anyone else who clones this repo

```

### Layer Responsibilities

| Layer | File | Responsibility |
|---|---|---|
| CLI | `cmd/root.go` | Command parsing, user I/O, calls manager |
| Business Logic | `wedev/manager.go` | Orchestration, rules enforcement, IP allocation |
| Persistence | `wedev/storage.go` | BoltDB bucket read/write; no business logic |
| Utilities | `util/util.go` | IP pool (`IPPool`), validators (`IPValidator`) |

### Key Types

- `VirtualNetworkManager` — top-level orchestrator in `wedev/manager.go`
- `StorageManager` — BoltDB wrapper in `wedev/storage.go`
- `IPPool` — allocates/recycles IPs from a CIDR in `util/util.go`
- `IPValidator` — validates network names, CIDRs, endpoints, ports in `util/util.go`

## Module & Toolchain

- **Module path**: `github.com/wedevctl`
- **Go version**: `go 1.25.0` / toolchain `go1.25.10` (see `go.mod`)
- Internal imports use the prefix `github.com/wedevctl/` (e.g. `github.com/wedevctl/cmd`, `github.com/wedevctl/wedev`, `github.com/wedevctl/util`)

## Dependencies

| Package | Version | Purpose |
|---|---|---|
| `github.com/spf13/cobra` | v1.10.2 | CLI framework |
| `go.etcd.io/bbolt` | v1.4.3 | Embedded key-value database |
| `github.com/google/uuid` | v1.6.0 | UUID generation |

## Configuration

- Default DB path: `~/.wedevctl/wedevctl.db`
- Override via env var: `WEDEVCTL_DB_PATH=<directory>`
- DB directory is created automatically with `0700` permissions

## Core Domain Concepts

- **Virtual Network**: isolated WireGuard network defined by a CIDR block (e.g. `10.0.0.0/24`)
- **Server**: single server per network; always gets the first IP in the CIDR; requires endpoint + listen port
- **Node types**:
  - `peer` — requires a public address; can communicate peer-to-peer
  - `route` — public address optional; communicates only via server
- **IP allocation**: sequential from CIDR; recycled on deletion
- **Config versioning**: each `config generate` is hash-tracked; history viewable with `config history`

## Validation Rules

- Network/node names: must start with a letter, followed by letters or digits only
  (alphanumeric — no hyphens, underscores, or other characters; see `IsValidNetworkName`)
- CIDR: standard IPv4 CIDR notation
- Peer nodes: public address is **required**
- Route nodes: public address is optional
- Changing a node type to `peer` requires a public address to be set

## Manual Smoke Tests

Two shell scripts are available for manual end-to-end smoke testing (not part of `go test`):

```bash
# Full smoke test: build, create network, add server + peer/route nodes, generate configs
bash diagnose.sh

# Multi-network scenario: same node names across networks, route nodes, etc.
bash test_scenarios.sh
```

Run these after significant changes to the config generation or network/node management logic.

## Performance Benchmarks

Go benchmarks live alongside the tests as `Benchmark*` functions in
`util/bench_test.go` and `wedev/bench_test.go`. They are **not run by `go test`**
and are **not a CI gate**. They are, however, run locally by the project Stop
hook (see *Local Verification Hook* below) whenever a hot-path file changes —
run them by hand too when changing a hot path:

```bash
# Run all benchmarks with allocation stats
go test -bench=. -benchmem ./util/... ./wedev/...

# Compare before/after a change (capture both, then diff with benchstat):
#   go test -bench=. -count=10 ./util/... > old.txt   (before the change)
#   go test -bench=. -count=10 ./util/... > new.txt   (after the change)
#   benchstat old.txt new.txt
```

Hot-path files — **changing any of these requires adding/refreshing a benchmark
or test** (enforced by the Stop hook):

- **IP allocation** (`util/util.go` — `AllocateNodeIP`, `SyncNextIndex`,
  `NewIPPool`) — O(1)/O(n) integer arithmetic; keep it that way (it was once
  O(n²) — see git history)
- **Config generation** (`wedev/manager.go` — `GenerateConfigs`) — each node's
  config enumerates every other node, so cost is inherently quadratic in node
  count; do not add further passes
- **Persistence** (`wedev/storage.go`) — per-network queries use the
  `*_by_name` / `*_by_network` / `configs_by_version` index buckets; never
  reintroduce a full `bucket.ForEach` scan for a per-network lookup

## Local Verification Hook

`.claude/settings.json` registers a **Stop hook** (`.claude/hooks/verify-changes.sh`)
that runs when a turn finishes. If the turn changed Go code it blocks completion
until:

- `go test -race ./...` passes and every package holds ≥80% coverage;
- `gofmt -s -l .`, `go vet ./...`, `go build ./...`, and `golangci-lint run`
  are all clean (this mirrors the CI static-analysis + build gate, so lint
  and format failures surface locally instead of only in GitHub Actions);
- when a hot-path file changed, a test/benchmark file was also updated and the
  benchmarks run clean.

The hook is local-only (it is not the CI itself). If `golangci-lint` is not
installed it warns and skips that gate rather than blocking — CI still enforces
it. Review or disable the hook via `/hooks`.

## Testing Requirements

All new code must include tests. Three tiers are required with 100% pass rate:

1. **Unit tests** — mock external dependencies where practical (filesystem, time)
2. **API/service tests** — test the service layer (`VirtualNetworkManager`) and storage
   (`StorageManager`) against a real BoltDB opened on a temp file via `t.TempDir()`.
   Tests are co-located as `*_test.go` files in each package — there is no shared
   `test/` fixture directory in this repo.
3. **E2E tests** — cover CLI user interaction flows by executing commands through
   `NewRootCommand()` (see `cmd/root_test.go`)

**Coverage gate**: ≥ 80% statements, enforced by CI both overall and per package
(`cmd`, `wedev`, `util`). A change that drops any of these below 80% must add tests.

After any code change, verify tests still pass:
```bash
go test -v -race ./...
```

If tests fail, fix the code first. Only adjust tests if the test itself is wrong, never delete failing tests.

## CI/CD Checks (must pass locally before PRs)

```bash
gofmt -s -l .         # No formatting diffs — CI fails if this lists any file
go vet ./...          # No vet errors
golangci-lint run     # Lint + security gate — go vet does NOT cover this
go test -race ./...   # All tests pass with race detector
go build ./...        # All packages compile cleanly
go build -o wedevctl main.go   # Produces the runnable binary (CI also runs `./wedevctl --help`)
govulncheck ./...     # No known CVEs in dependencies or the Go toolchain
```

`go build ./...` is the broader gate — it compiles every package; `go build -o wedevctl
main.go` only builds what `main` imports but emits the actual binary. CI runs both.

`golangci-lint run` is **not optional and not redundant with `go vet`** — its
`errcheck`, `gocritic`, `gosec` (and other) linters catch a large class of issues
`go vet` ignores. The Stop hook runs it automatically; run it by hand too.
Install once with `go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest`,
and `govulncheck` with `go install golang.org/x/vuln/cmd/govulncheck@latest`.

The GitHub Actions pipeline (`.github/workflows/pr-checks.yml`) runs the same
checks, plus:

- **golangci-lint** (pinned `v2.11.4`, config in `.golangci.yml`) — the blocking
  lint + security gate. `gosec` runs *inside* golangci-lint, so code-level security
  findings fail this step.
- **gosec** (standalone job) — runs with `-no-fail` and uploads SARIF to the GitHub
  Security tab; it is for reporting and does not block on its own.
- **govulncheck** (standalone step) — scans dependencies *and* the Go stdlib/toolchain
  for known CVEs, with reachability analysis. This step **does** block: a known CVE
  in a reachable code path fails the build.

`gosec` covers insecure patterns in *our* code; `govulncheck` covers known
vulnerabilities in *dependencies and the toolchain* — they do not overlap.

Keep code lint-clean and free of security issues (OWASP Top 10).

## Code Conventions

- All code, comments, and documentation must be in **English**
- Follow standard Go idioms and naming conventions
- Use `fmt.Errorf("...: %w", err)` for error wrapping
- DB operations belong in `storage.go`; no BoltDB calls in `manager.go` or `cmd/`
- CLI output goes to `os.Stdout`; errors go to `os.Stderr`
- No business logic in `cmd/root.go` — delegate to `VirtualNetworkManager`
- Do not add features, refactors, or optimizations beyond what is explicitly requested

## Workflow Standards

See [Workflow Standards](.claude/workflow.md) for full testing and development rules.
