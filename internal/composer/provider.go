package composer

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/JBSommeling/dependency-curator/internal/dependency"
)

type ComposerProvider struct{}

func NewComposerProvider() *ComposerProvider {
	return &ComposerProvider{}
}

func (p *ComposerProvider) Name() string {
	return "composer"
}

type composerJSON struct {
	Require    map[string]string `json:"require"`
	RequireDev map[string]string `json:"require-dev"`
}

func (p *ComposerProvider) Discover(projectDir string) ([]dependency.Dependency, error) {
	path := filepath.Join(projectDir, "composer.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading composer.json: %w", err)
	}

	var pkg composerJSON
	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil, fmt.Errorf("parsing composer.json: %w", err)
	}

	var deps []dependency.Dependency

	for name, version := range pkg.Require {
		if isPlatformRequirement(name) {
			continue
		}
		deps = append(deps, dependency.Dependency{
			Name:           name,
			CurrentVersion: cleanVersion(version),
			IsDev:          false,
		})
	}

	for name, version := range pkg.RequireDev {
		if isPlatformRequirement(name) {
			continue
		}
		deps = append(deps, dependency.Dependency{
			Name:           name,
			CurrentVersion: cleanVersion(version),
			IsDev:          true,
		})
	}

	return deps, nil
}

func isPlatformRequirement(name string) bool {
	if name == "php" || name == "hhvm" {
		return true
	}
	if strings.HasPrefix(name, "ext-") {
		return true
	}
	if strings.HasPrefix(name, "lib-") {
		return true
	}
	if name == "composer-plugin-api" || name == "composer-runtime-api" {
		return true
	}
	return false
}

func cleanVersion(v string) string {
	v = strings.TrimSpace(v)
	for _, prefix := range []string{">=", "<=", "^", "~", ">", "<", "=", "v"} {
		v = strings.TrimPrefix(v, prefix)
	}
	// Handle OR constraints like "^7.0 || ^8.0" — take the first
	if idx := strings.Index(v, "||"); idx != -1 {
		v = strings.TrimSpace(v[:idx])
		// Re-strip prefixes from first part
		for _, prefix := range []string{">=", "<=", "^", "~", ">", "<", "=", "v"} {
			v = strings.TrimPrefix(v, prefix)
		}
	}
	// Handle range constraints like ">=7.0 <8.0" — take the first
	if idx := strings.Index(v, " "); idx != -1 {
		v = v[:idx]
	}
	return strings.TrimSpace(v)
}
