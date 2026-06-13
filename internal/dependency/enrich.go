package dependency

func Enrich(deps []Dependency, updates []UpdateInfo, advisories []AdvisoryInfo) []Dependency {
	updateMap := make(map[string]UpdateInfo, len(updates))
	for _, u := range updates {
		updateMap[u.Name] = u
	}

	advisoryMap := make(map[string][]AdvisoryInfo)
	for _, a := range advisories {
		advisoryMap[a.Package] = append(advisoryMap[a.Package], a)
	}

	result := make([]Dependency, len(deps))
	for i, dep := range deps {
		enriched := dep

		if u, ok := updateMap[dep.Name]; ok {
			enriched.LatestVersion = u.Latest
			enriched.UpdateType = u.UpdateType
		}

		if advs, ok := advisoryMap[dep.Name]; ok {
			for _, a := range advs {
				enriched.Advisories = append(enriched.Advisories, Advisory{
					ID:               a.ID,
					Severity:         a.Severity,
					Title:            a.Title,
					AffectedVersions: a.AffectedVersions,
					FixedVersion:     a.FixedVersion,
					URL:              a.URL,
				})
			}
		}

		result[i] = enriched
	}

	return result
}
