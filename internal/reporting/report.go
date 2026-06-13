package reporting

import (
	"fmt"
	"sort"
	"strings"

	"github.com/JBSommeling/dependency-curator/internal/changelog"
	"github.com/JBSommeling/dependency-curator/internal/dependency"
)

type Report struct {
	PatchCount    int
	MinorCount    int
	MajorCount    int
	VulnCount     int
	PatchDeps     []dependency.Dependency
	MinorDeps     []dependency.Dependency
	MajorDeps     []dependency.Dependency
	AllAdvisories []dependency.Advisory
	Changelogs    map[string]*changelog.ChangelogInfo
}

func BuildReport(deps []dependency.Dependency, changelogs map[string]*changelog.ChangelogInfo) *Report {
	r := &Report{
		Changelogs: changelogs,
	}

	for _, d := range deps {
		switch d.UpdateType {
		case "patch":
			r.PatchCount++
			r.PatchDeps = append(r.PatchDeps, d)
		case "minor":
			r.MinorCount++
			r.MinorDeps = append(r.MinorDeps, d)
		case "major":
			r.MajorCount++
			r.MajorDeps = append(r.MajorDeps, d)
		}
		for _, a := range d.Advisories {
			r.VulnCount++
			r.AllAdvisories = append(r.AllAdvisories, a)
		}
	}

	return r
}

func GeneratePRBody(deps []dependency.Dependency, changelogs map[string]*changelog.ChangelogInfo) string {
	report := BuildReport(deps, changelogs)
	var b strings.Builder

	b.WriteString("# Dependency Guardian Report\n\n")

	// Executive Summary
	b.WriteString("## Executive Summary\n\n")
	total := report.PatchCount + report.MinorCount + report.MajorCount
	b.WriteString(fmt.Sprintf("| Metric | Count |\n"))
	b.WriteString(fmt.Sprintf("|--------|-------|\n"))
	b.WriteString(fmt.Sprintf("| Total updates | %d |\n", total))
	b.WriteString(fmt.Sprintf("| Patch updates | %d |\n", report.PatchCount))
	b.WriteString(fmt.Sprintf("| Minor updates | %d |\n", report.MinorCount))
	b.WriteString(fmt.Sprintf("| Major updates | %d |\n", report.MajorCount))
	b.WriteString(fmt.Sprintf("| Vulnerabilities fixed | %d |\n\n", report.VulnCount))

	// Security Impact
	if len(report.AllAdvisories) > 0 {
		b.WriteString("## Security Impact\n\n")
		b.WriteString("| Package | Severity | Advisory | Title |\n")
		b.WriteString("|---------|----------|----------|-------|\n")

		// Sort by severity (critical first)
		sortedAdvisories := make([]advisoryWithPkg, 0)
		for _, d := range deps {
			for _, a := range d.Advisories {
				sortedAdvisories = append(sortedAdvisories, advisoryWithPkg{pkg: d.Name, advisory: a})
			}
		}
		sort.Slice(sortedAdvisories, func(i, j int) bool {
			return severityRank(sortedAdvisories[i].advisory.Severity) < severityRank(sortedAdvisories[j].advisory.Severity)
		})

		for _, sa := range sortedAdvisories {
			a := sa.advisory
			id := a.ID
			if a.URL != "" {
				id = fmt.Sprintf("[%s](%s)", a.ID, a.URL)
			}
			b.WriteString(fmt.Sprintf("| %s | %s | %s | %s |\n", sa.pkg, a.Severity, id, a.Title))
		}
		b.WriteString("\n")
	}

	// Updated Dependencies
	allUpdated := make([]dependency.Dependency, 0)
	allUpdated = append(allUpdated, report.MajorDeps...)
	allUpdated = append(allUpdated, report.MinorDeps...)
	allUpdated = append(allUpdated, report.PatchDeps...)

	if len(allUpdated) > 0 {
		b.WriteString("## Updated Dependencies\n\n")
		b.WriteString("| Package | From | To | Type |\n")
		b.WriteString("|---------|------|----|------|\n")
		for _, d := range allUpdated {
			b.WriteString(fmt.Sprintf("| %s | %s | %s | %s |\n", d.Name, d.CurrentVersion, d.LatestVersion, d.UpdateType))
		}
		b.WriteString("\n")
	}

	// Breaking Change Analysis
	if report.MajorCount > 0 {
		b.WriteString("## Breaking Change Analysis\n\n")
		for _, d := range report.MajorDeps {
			b.WriteString(fmt.Sprintf("### %s\n\n", d.Name))
			b.WriteString(fmt.Sprintf("**Version:** %s → %s\n\n", d.CurrentVersion, d.LatestVersion))

			if cl, ok := changelogs[d.Name]; ok && cl.Available {
				if len(cl.BreakingChanges) > 0 {
					b.WriteString("**Potential Breaking Changes:**\n\n")
					for _, bc := range cl.BreakingChanges {
						b.WriteString(fmt.Sprintf("- %s\n", bc))
					}
					b.WriteString("\n")
				}
				if len(cl.MigrationNotes) > 0 {
					b.WriteString("**Migration Notes:**\n\n")
					for _, mn := range cl.MigrationNotes {
						b.WriteString(fmt.Sprintf("- %s\n", mn))
					}
					b.WriteString("\n")
				}
				if cl.ReleaseNotesURL != "" || cl.ChangelogURL != "" {
					b.WriteString("**References:**\n\n")
					if cl.ReleaseNotesURL != "" {
						b.WriteString(fmt.Sprintf("- [Release Notes](%s)\n", cl.ReleaseNotesURL))
					}
					if cl.ChangelogURL != "" {
						b.WriteString(fmt.Sprintf("- [Changelog](%s)\n", cl.ChangelogURL))
					}
					b.WriteString("\n")
				}
			} else {
				b.WriteString("_Changelog information not available._\n\n")
			}
		}
	}

	// Validation Checklist
	b.WriteString("## Validation Checklist\n\n")
	b.WriteString("- [ ] Install dependencies\n")
	b.WriteString("- [ ] Run unit tests\n")
	b.WriteString("- [ ] Run integration tests\n")
	b.WriteString("- [ ] Verify CI pipeline\n")
	b.WriteString("- [ ] Verify application startup\n\n")

	// Risk Assessment
	b.WriteString("## Risk Assessment\n\n")
	risk, reason := assessRisk(report)
	b.WriteString(fmt.Sprintf("**Risk Level: %s**\n\n", risk))
	b.WriteString(fmt.Sprintf("%s\n", reason))

	return b.String()
}

func GenerateSummary(deps []dependency.Dependency, patchesApplied int) string {
	report := BuildReport(deps, nil)
	var b strings.Builder

	b.WriteString("## Dependency Guardian Summary\n\n")
	if patchesApplied > 0 {
		b.WriteString(fmt.Sprintf("- **%d** patch updates applied\n", patchesApplied))
	}
	if report.MinorCount > 0 {
		b.WriteString(fmt.Sprintf("- **%d** minor updates available\n", report.MinorCount))
	}
	if report.MajorCount > 0 {
		b.WriteString(fmt.Sprintf("- **%d** major updates available\n", report.MajorCount))
	}
	if report.VulnCount > 0 {
		b.WriteString(fmt.Sprintf("- **%d** vulnerabilities found\n", report.VulnCount))
	}
	if patchesApplied == 0 && report.MinorCount == 0 && report.MajorCount == 0 {
		b.WriteString("All dependencies are up to date.\n")
	}

	return b.String()
}

type advisoryWithPkg struct {
	pkg      string
	advisory dependency.Advisory
}

func severityRank(s string) int {
	switch s {
	case "critical":
		return 0
	case "high":
		return 1
	case "moderate":
		return 2
	case "low":
		return 3
	case "info":
		return 4
	default:
		return 5
	}
}

func assessRisk(r *Report) (string, string) {
	if r.MajorCount > 0 {
		hasVulns := r.VulnCount > 0
		if hasVulns {
			return "High", "Major version updates with known vulnerabilities require careful review and testing."
		}
		return "Medium", "Major version updates may contain breaking changes. Review the breaking change analysis above."
	}
	if r.VulnCount > 0 {
		return "Medium", "Security vulnerabilities detected. Updating is recommended."
	}
	if r.MinorCount > 0 {
		return "Low", "Minor updates typically add features without breaking changes."
	}
	return "Low", "Only patch updates detected. These are typically safe to apply."
}
