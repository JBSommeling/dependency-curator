package gomod

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/JBSommeling/dependency-curator/internal/dependency"
)

type Provider struct{}

func NewProvider() *Provider {
	return &Provider{}
}

func (p *Provider) Name() string {
	return "gomod"
}

func (p *Provider) Discover(projectDir string) ([]dependency.Dependency, error) {
	path := filepath.Join(projectDir, "go.mod")
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("reading go.mod: %w", err)
	}
	defer f.Close()

	var deps []dependency.Dependency
	inRequireBlock := false

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip indirect dependencies
		if strings.Contains(line, "// indirect") {
			continue
		}

		// Strip other inline comments
		if idx := strings.Index(line, "//"); idx != -1 {
			line = strings.TrimSpace(line[:idx])
		}

		if line == "require (" || line == "require(" {
			inRequireBlock = true
			continue
		}
		if inRequireBlock && line == ")" {
			inRequireBlock = false
			continue
		}

		var entry string
		if inRequireBlock {
			entry = line
		} else if strings.HasPrefix(line, "require ") {
			entry = strings.TrimSpace(strings.TrimPrefix(line, "require "))
		} else {
			continue
		}

		parts := strings.Fields(entry)
		if len(parts) < 2 {
			continue
		}

		deps = append(deps, dependency.Dependency{
			Name:           parts[0],
			CurrentVersion: strings.TrimPrefix(parts[1], "v"),
			IsDev:          false,
		})
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scanning go.mod: %w", err)
	}

	return deps, nil
}
