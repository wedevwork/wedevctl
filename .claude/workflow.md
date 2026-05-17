# Development Workflow Standards

This file defines the **required engineering workflow** for all development tasks in this project.
Claude Code must load and adhere to these standards throughout the entire development lifecycle.

---

## Rule 1: Test-First, Always

- Never deliver a code change without corresponding tests.
- Write or update tests **in the same step** as the code change, not as a follow-up.
- Mock **all** external dependencies (APIs, databases, file systems, time) in unit tests.
- Treat untested code as unfinished code.

## Rule 2: Coverage & Pass Rate Are Non-Negotiable

Run the full unit test suite after every change. Both of the following must be true before moving on:

- **Pass rate**: 100% — any failing test blocks further work.
- **Coverage**: ≥ 80% statements — enforced by CI both overall **and per package**
  (`cmd`, `wedev`, `util`). If a change causes any of these to drop below 80%, add
  tests immediately.

These two rules are also enforced locally: a **Stop hook** (`.claude/settings.json`
→ `.claude/hooks/verify-changes.sh`) runs at the end of any turn that changed Go
code and blocks completion until the suite passes and coverage holds. The hook is
the mechanical backstop for this rule — it does not replace writing the tests as
part of the change.

## Rule 3: Tiered Testing — Know When to Escalate

Do not run everything every time, but do not skip either. Use this decision framework:

```
Every change
  └─► Unit tests + API mock tests          ← always required

  If CLI contracts or shared modules changed (e.g., StorageManager, VirtualNetworkManager interfaces):
  └─► + service-layer integration tests (real BoltDB, temp file DB)

  If high-risk area or pre-release:
  └─► + Full E2E regression (diagnose.sh, test_scenarios.sh)
```

High-risk areas include: IP pool allocation logic, WireGuard config generation (Endpoint
inclusion rules for peer vs route nodes), BoltDB transaction consistency, cascade deletion,
and CIDR/IP validation logic.

Several high-risk areas are also **performance hot paths** — when a change
touches one of those files, Rule 6 (the benchmark flow) applies in addition to
the testing tiers above.

## Rule 4: Follow the Structured Development Sequence

For every feature, fix, or refactor, work through these phases in order:

```
[ Clarify ] → [ Plan ] → [ Generate ] → [ Test & Refine ] → [ Validate in CI ]
```

- **Clarify**: Discuss requirements, confirm scope, surface edge cases.
- **Plan**: Design the implementation — interfaces, data flow, dependencies.
- **Generate**: Write production code + unit tests + mocks + integration/E2E cases together.
- **Test & Refine**: Run all tests locally; iterate until all gates pass.
- **Validate in CI**: Push and confirm the full pipeline passes independently.

Each phase must be complete before the next begins.
Do not jump to implementation without completing the clarification and planning phases.

## Rule 5: Change Impact Analysis Before Escalating Tests

Before running integration or E2E tests, explicitly assess:

- **Scope**: How many modules / services are affected?
- **Risk**: Does the change touch any high-risk area (see Rule 3)?
- **Dependencies**: Are any API contracts or shared interfaces modified?

Document the assessment briefly in your task notes or commit message when escalating to full regression.

## Rule 6: Benchmark the Hot Paths

Some files are performance-sensitive enough that a change to them must be
*measured*, not just tested for correctness. The **hot-path files** are:

- `util/util.go` — IP pool allocation and address arithmetic
- `wedev/manager.go` — WireGuard config generation
- `wedev/storage.go` — BoltDB persistence and indexed lookups

When a change touches any of them, in the same step as the change:

1. **Add or update a `Benchmark*` case** covering the changed path, in
   `util/bench_test.go` or `wedev/bench_test.go`.
2. **Run the benchmarks** and read the result:
   ```
   go test -bench=. -benchmem ./util/... ./wedev/...
   ```
3. **If the change could affect cost**, capture before/after and compare:
   ```
   go test -bench=. -count=10 ./wedev/... > old.txt   # before the change
   go test -bench=. -count=10 ./wedev/... > new.txt   # after the change
   benchstat old.txt new.txt
   ```
   Watch the size sub-benchmarks (e.g. `n=100` → `n=1000`): cost growing faster
   than the input is the signal that a change regressed an algorithm.

**What is enforced vs. not** — be honest about the boundary:

- The Stop hook (`.claude/hooks/verify-changes.sh`) **does** block completion if
  a hot-path file changed without a `*_test.go` update, or if the benchmarks
  fail to build/run or exceed their timeout (a pathological-regression tripwire).
- It does **not** detect a gradual slowdown — there is no committed baseline.
  Catching small regressions is a judgement call from the `benchstat` output
  above, made during "Test & Refine" (Rule 4).

Keep the hot paths at their current complexity: IP allocation is O(1)/O(n)
integer arithmetic, and storage lookups are index-backed. Do not reintroduce a
full bucket scan or a per-call recompute.
