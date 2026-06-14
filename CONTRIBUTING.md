# Contributing to Dependency Curator

Thank you for your interest in contributing!

## Development Setup

### Prerequisites

- Go 1.24+
- Docker (for building the action image)
- Node.js 22+ and npm (for npm ecosystem testing)
- PHP 8.3+ and Composer (for Composer ecosystem testing)

### Getting Started

1. Clone the repository:
   ```bash
   git clone https://github.com/JBSommeling/dependency-curator.git
   cd dependency-curator
   ```

2. Install dependencies:
   ```bash
   go mod download
   ```

3. Run tests:
   ```bash
   make test
   ```

4. Build the binary:
   ```bash
   make build
   ```

5. Build the Docker image:
   ```bash
   make docker
   ```

## Project Structure

```
cmd/action/          Main entrypoint and orchestrator
internal/
  config/            Action input parsing
  dependency/        Dependency model, enrichment, and provider interface
  scanner/           npm outdated parsing
  security/          npm audit parsing
  updater/           npm patch update application
  composer/          Composer provider, scanner, security, and updater
  gomod/             Go modules provider, scanner, security, and updater
  reporting/         Markdown report generation
  github/            GitHub API client
  changelog/         Changelog metadata collection
  exec/              Command execution abstraction
pkg/
  semver/            Semver parsing and comparison
```

## Adding a New Ecosystem

Dependency Curator is designed to support multiple ecosystems. To add support for a new package manager:

1. Implement the `dependency.Provider` interface in a new package
2. Implement the equivalent scanner, security, and updater logic
3. Register the new provider in the orchestrator
4. Add tests with fixture files

## Pull Requests

- Write tests for new functionality
- Run `make test` before submitting
- Keep commits focused and atomic
- Follow existing code style

## Reporting Issues

Please use GitHub Issues to report bugs or request features. Include:

- Steps to reproduce
- Expected behavior
- Actual behavior
- Go version and OS
