package dependency

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type PackageJSONProvider struct{}

func NewPackageJSONProvider() *PackageJSONProvider {
	return &PackageJSONProvider{}
}

func (p *PackageJSONProvider) Name() string {
	return "npm"
}

type packageJSON struct {
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
}

func (p *PackageJSONProvider) Discover(projectDir string) ([]Dependency, error) {
	path := filepath.Join(projectDir, "package.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading package.json: %w", err)
	}

	var pkg packageJSON
	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil, fmt.Errorf("parsing package.json: %w", err)
	}

	var deps []Dependency

	for name, version := range pkg.Dependencies {
		deps = append(deps, Dependency{
			Name:           name,
			CurrentVersion: cleanVersion(version),
			IsDev:          false,
		})
	}

	for name, version := range pkg.DevDependencies {
		deps = append(deps, Dependency{
			Name:           name,
			CurrentVersion: cleanVersion(version),
			IsDev:          true,
		})
	}

	return deps, nil
}

func cleanVersion(v string) string {
	v = strings.TrimSpace(v)
	// Strip range prefixes
	for _, prefix := range []string{">=", "<=", "^", "~", ">", "<", "=", "v"} {
		v = strings.TrimPrefix(v, prefix)
	}
	return strings.TrimSpace(v)
}
