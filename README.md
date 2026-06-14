# Dependency Curator

A GitHub Action that automatically maintains project dependencies across multiple ecosystems while minimizing pull request noise.

## Why?

Tools like Dependabot and Renovate create dozens of individual PRs for routine dependency updates. This fragments review effort, clutters notifications, and discourages teams from staying current.

**Dependency Curator takes a different approach:**

- Automatically applies safe patch updates in a single commit
- Creates one consolidated PR for all minor and major updates
- Generates a detailed report with security advisories and breaking changes
- Supports multiple ecosystems (npm, Composer, Go) in a single run
- Reduces maintenance burden to reviewing a single, well-documented PR

## Quick Start

Add this workflow to your repository at `.github/workflows/dependency-curator.yml`:

```yaml
name: Dependency Curator
on:
  schedule:
    - cron: "0 6 * * 1"  # Every Monday at 6 AM
  workflow_dispatch:       # Allow manual trigger

permissions:
  contents: write
  pull-requests: write

jobs:
  update-dependencies:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: JBSommeling/dependency-curator@v1.1.1
        with:
          github_token: ${{ secrets.GITHUB_TOKEN }}
```

## What It Does

On each run, Dependency Curator:

1. Auto-detects supported ecosystems in your project
2. Discovers dependencies from each ecosystem's manifest
3. Checks for available updates and scans for vulnerabilities
4. Classifies updates as patch, minor, or major
5. **Patch updates**: Automatically applied and committed (per ecosystem)
6. **Minor + Major updates**: Grouped into a single consolidated PR with a detailed report

### Supported Ecosystems

| Ecosystem | Manifest | Updates | Vulnerabilities |
|-----------|----------|---------|-----------------|
| **npm** | `package.json` | `npm outdated` | `npm audit` |
| **Composer** | `composer.json` | `composer outdated` | `composer audit` |
| **Go Modules** | `go.mod` | `go list -m -u` | `govulncheck` |

All detected ecosystems are processed in a single run and combined into one PR.

## PR Report

The generated PR includes:

- **Executive Summary** with update counts
- **Security Impact** table with vulnerability details
- **Updated Dependencies** table
- **Breaking Change Analysis** for major updates (with changelog links)
- **Validation Checklist** for manual review
- **Risk Assessment** (Low / Medium / High)

## Inputs

| Input | Default | Description |
|-------|---------|-------------|
| `github_token` | `${{ github.token }}` | GitHub token for API access |
| `auto_patch` | `true` | Automatically apply patch updates |
| `create_pr` | `true` | Create PR for minor/major updates |
| `include_dev_dependencies` | `true` | Include devDependencies |
| `schedule_label` | `weekly` | Label for branch naming |
| `risk_threshold` | `high` | Max risk level for auto-patching |
| `base_branch` | _(repo default)_ | Target branch for PRs |

## Outputs

| Output | Description |
|--------|-------------|
| `patches_applied` | Number of patch updates applied |
| `updates_available` | Number of minor/major updates available |
| `vulnerabilities_found` | Number of vulnerabilities found |
| `pr_url` | URL of the created/updated PR |

## Permissions

The action requires these permissions:

```yaml
permissions:
  contents: write        # For creating branches and commits
  pull-requests: write   # For creating and updating PRs
```

## Architecture

Dependency Curator is written in Go and runs as a Docker-based GitHub Action. It uses a clean architecture with interfaces designed for ecosystem extensibility.

```
Discover → Scan → Enrich → Update → Report → PR
```

Each step is a separate, testable module. See [CONTRIBUTING.md](CONTRIBUTING.md) for details.

## Roadmap

- [x] Composer (PHP) support
- [x] Go Modules support
- [ ] Python (pip/poetry) support
- [ ] Rust (Cargo) support
- [ ] Monorepo support
- [ ] Slack/Teams notifications
- [ ] GitHub App mode
- [ ] Repository health scoring

## License

MIT License. See [LICENSE](LICENSE) for details.
