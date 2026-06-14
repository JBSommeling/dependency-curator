## Summary

Implements Dependency Curator — a Docker-based GitHub Action that automatically maintains Node.js project dependencies while minimizing pull request noise.

Unlike Dependabot and Renovate which create dozens of individual PRs, Dependency Curator:

- **Auto-applies patch updates** in a single commit
- **Creates one consolidated PR** for all minor and major updates
- **Generates a structured report** with security advisories, breaking changes, and risk assessment
- **Scans for vulnerabilities** via `npm audit`

## Architecture

```
Discover → Scan → Enrich → Update → Report → PR
```

| Package | Responsibility |
|---------|---------------|
| `cmd/action/` | Orchestrator — wires all components |
| `internal/config/` | Parse GitHub Action inputs from env vars |
| `internal/dependency/` | Dependency model, package.json discovery, enrichment |
| `internal/scanner/` | `npm outdated` parsing with semver classification |
| `internal/security/` | `npm audit` v2 JSON parsing |
| `internal/updater/` | Apply patch-level `npm install` |
| `internal/reporting/` | Markdown PR body and summary generation |
| `internal/github/` | GitHub REST API client (branches, commits, PRs) |
| `internal/changelog/` | npm registry metadata for breaking change URLs |
| `internal/exec/` | `CommandRunner` interface for testable command execution |
| `pkg/semver/` | Semver parsing, comparison, update classification |

Interfaces at every boundary (`DependencyProvider`, `VulnerabilityScanner`, `CommandRunner`, `HTTPClient`, `ChangelogProvider`) make future ecosystem support (Composer, Go Modules, Python, Rust) addable without refactoring.

## What's included

- **40 files**, ~4,500 lines of Go
- **85+ tests** across 10 packages (all passing with `-race`)
- `action.yml` with 7 configurable inputs and 4 outputs
- Multi-stage `Dockerfile` (Go builder → Node 22 Alpine runtime)
- CI workflow (`.github/workflows/ci.yml`)
- `README.md` with usage guide, input/output tables, roadmap
- `CONTRIBUTING.md` with development setup
- MIT `LICENSE`

## Key design decisions

- **`Run` vs `RunAllowExit1`** — npm returns exit code 1 for `outdated`/`audit` when results exist; the runner explicitly separates strict vs tolerant execution
- **Base64 blob encoding** — `CreateBlob` uses base64 for binary-safe Git tree commits
- **Attempt-then-handle branching** — avoids TOCTOU race on branch creation; stale branches are fast-forwarded via `UpdateRef`
- **`--` separator in `npm install`** — prevents flag injection from crafted package names
- **`--omit=dev` at npm level** — dev dependency filtering happens at scan time, not post-hoc
- **No commit on partial failure** — if any patch install fails, the lockfile isn't committed in an inconsistent state
- **Response size limits** — `io.LimitReader` on all HTTP responses (GitHub API: 10MB, npm registry: 5MB)

## Usage

```yaml
name: Dependency Curator
on:
  schedule:
    - cron: "0 6 * * 1"
  workflow_dispatch:

permissions:
  contents: write
  pull-requests: write

jobs:
  update-dependencies:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: JBSommeling/dependency-curator@latest
        with:
          github_token: ${{ secrets.GITHUB_TOKEN }}
```

## Test plan

- [x] All 85+ unit tests pass with `-race`
- [x] `go build`, `go vet` clean
- [x] Integration tests verify full orchestration flow (no-deps, with-updates, existing-PR paths)
- [x] 4-iteration code review cycle converged with 0 residuals
- [ ] Manual: install in a test repo and trigger via `workflow_dispatch`
- [ ] Manual: verify Docker image builds in CI
