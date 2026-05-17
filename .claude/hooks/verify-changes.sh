#!/usr/bin/env bash
# verify-changes.sh — Stop hook for wedevctl.
#
# Runs once when Claude finishes a turn. If the turn touched Go code it
# enforces, locally, the rules in .claude/workflow.md:
#   * go test -race ./...      passes 100%
#   * cmd / wedev / util       keep >= 80% statement coverage
#   * gofmt -s / go vet / go build ./... / golangci-lint  all clean
#     (mirrors the CI static-analysis + build gate, so lint failures are
#     caught locally instead of only in GitHub Actions)
#   * a hot-path change is accompanied by a *_test.go change AND the
#     benchmarks still run clean (catches pathological slowdowns).
#
# Exit 0  -> checks passed, allow the stop.
# Exit 2  -> checks failed, block the stop; stderr is fed back to Claude.
# Exit 0 is also used for any environment problem (not in a repo, etc.) so
# the hook never wedges a session on infrastructure issues.

set -uo pipefail

ROOT="$(git rev-parse --show-toplevel 2>/dev/null)" || exit 0
cd "$ROOT" || exit 0

# This environment exports a stale GOROOT; let the go toolchain use its own.
unset GOROOT

COVERAGE_MIN=80
HOTPATHS=("util/util.go" "wedev/manager.go" "wedev/storage.go")

# --- collect changed Go files (uncommitted diff vs HEAD + untracked) ------
changed="$(
  {
    git diff --name-only HEAD -- '*.go'
    git ls-files --others --exclude-standard -- '*.go'
  } 2>/dev/null | sort -u
)"

# No Go changes this turn -> nothing to verify.
[ -z "$changed" ] && exit 0

fail=""

# --- 1. test suite + per-package coverage ---------------------------------
test_out="$(go test -race -cover ./... 2>&1)"
if [ $? -ne 0 ]; then
  fail="$fail
[tests] 'go test -race ./...' FAILED:
$test_out"
else
  while IFS= read -r line; do
    pkg="$(awk '{print $2}' <<<"$line")"
    cov="$(grep -oE 'coverage: [0-9.]+%' <<<"$line" | grep -oE '[0-9.]+')"
    case "$pkg" in
      */cmd | */wedev | */util)
        if [ -n "$cov" ] && awk "BEGIN{exit !($cov < $COVERAGE_MIN)}"; then
          fail="$fail
[coverage] package $pkg is at ${cov}% (below required ${COVERAGE_MIN}%)"
        fi
        ;;
    esac
  done <<<"$(grep 'coverage:' <<<"$test_out")"
fi

# --- 2. static analysis + build (mirrors the CI golangci-lint/build gate) -
fmt_out="$(gofmt -s -l . 2>&1)"
if [ -n "$fmt_out" ]; then
  fail="$fail
[gofmt] the following files are not 'gofmt -s' formatted:
$fmt_out
  Fix with: gofmt -s -w ."
fi

vet_out="$(go vet ./... 2>&1)"
if [ $? -ne 0 ]; then
  fail="$fail
[go vet] 'go vet ./...' reported problems:
$vet_out"
fi

build_out="$(go build ./... 2>&1)"
if [ $? -ne 0 ]; then
  fail="$fail
[build] 'go build ./...' FAILED:
$build_out"
fi

# golangci-lint is the blocking CI lint+security gate (gosec runs inside it).
# If it is not installed, warn but do not block — consistent with the hook's
# rule of never wedging a session on a missing tool; CI still enforces it.
if command -v golangci-lint >/dev/null 2>&1; then
  lint_out="$(golangci-lint run --timeout=2m ./... 2>&1)"
  if [ $? -ne 0 ]; then
    fail="$fail
[golangci-lint] lint/security gate reported issues:
$lint_out"
  fi
else
  echo "verify-changes.sh: warning — golangci-lint not installed; lint gate skipped (CI still enforces it). Install: go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest" >&2
fi

# --- 3. hot-path benchmark gate -------------------------------------------
hot_hit=""
for hp in "${HOTPATHS[@]}"; do
  grep -qxF "$hp" <<<"$changed" && hot_hit="$hot_hit $hp"
done

if [ -n "$hot_hit" ]; then
  if ! grep -qE '_test\.go$' <<<"$changed"; then
    fail="$fail
[benchmark] hot-path file(s) changed:$hot_hit
  but no *_test.go was added/updated. Add or update benchmark/test cases
  for the changed hot path (see CLAUDE.md > Performance Benchmarks)."
  fi

  bench_out="$(go test -bench=. -benchmem -benchtime=1x -run='^$' \
    -timeout=120s ./util/... ./wedev/... 2>&1)"
  if [ $? -ne 0 ]; then
    fail="$fail
[benchmark] benchmark run FAILED (build error, panic, or >120s timeout —
a likely sign of a pathological performance regression):
$bench_out"
  fi
fi

# --- verdict --------------------------------------------------------------
if [ -n "$fail" ]; then
  {
    echo "Stop hook: local verification failed — fix the following before finishing:"
    echo "$fail"
  } >&2
  exit 2
fi

exit 0
