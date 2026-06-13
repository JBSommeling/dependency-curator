package changelog

import "context"

type ChangelogInfo struct {
	PackageName     string
	FromVersion     string
	ToVersion       string
	BreakingChanges []string
	MigrationNotes  []string
	ReleaseNotesURL string
	ChangelogURL    string
	Available       bool
}

type Provider interface {
	FetchChangelog(ctx context.Context, pkg string, fromVer, toVer string) (*ChangelogInfo, error)
}
