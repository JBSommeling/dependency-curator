package dependency

type Dependency struct {
	Name           string
	CurrentVersion string
	LatestVersion  string
	UpdateType     string // "patch", "minor", "major", "none"
	IsDev          bool
	Advisories     []Advisory
}

type Advisory struct {
	ID               string
	Severity         string // "critical", "high", "moderate", "low", "info"
	Title            string
	AffectedVersions string
	FixedVersion     string
	URL              string
}

type Provider interface {
	Discover(projectDir string) ([]Dependency, error)
	Name() string
}
