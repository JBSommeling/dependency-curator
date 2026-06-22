package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/JBSommeling/dependency-curator/internal/changelog"
	"github.com/JBSommeling/dependency-curator/internal/composer"
	"github.com/JBSommeling/dependency-curator/internal/config"
	"github.com/JBSommeling/dependency-curator/internal/dependency"
	"github.com/JBSommeling/dependency-curator/internal/exec"
	gh "github.com/JBSommeling/dependency-curator/internal/github"
	"github.com/JBSommeling/dependency-curator/internal/gomod"
	"github.com/JBSommeling/dependency-curator/internal/reporting"
	"github.com/JBSommeling/dependency-curator/internal/scanner"
	"github.com/JBSommeling/dependency-curator/internal/security"
	"github.com/JBSommeling/dependency-curator/internal/updater"
)

type ghClientInterface interface {
	GetDefaultBranch(ctx context.Context, owner, repo string) (string, error)
	GetRef(ctx context.Context, owner, repo, ref string) (string, error)
	BranchExists(ctx context.Context, owner, repo, branch string) (bool, error)
	CreateBranch(ctx context.Context, owner, repo, branch, fromSHA string) error
	UpdateRef(ctx context.Context, owner, repo, ref, sha string) error
	CommitFiles(ctx context.Context, owner, repo, branch, message string, files map[string][]byte) (string, error)
	FindOpenPR(ctx context.Context, owner, repo, head, base string) (int, bool, error)
	CreatePR(ctx context.Context, owner, repo string, pr gh.PRRequest) (string, int, error)
	UpdatePR(ctx context.Context, owner, repo string, prNumber int, pr gh.PRRequest) error
	AddLabels(ctx context.Context, owner, repo string, prNumber int, labels []string) error
	MergePR(ctx context.Context, owner, repo string, prNumber int, method string) error
}

type deps struct {
	runner     exec.CommandRunner
	ghClient   ghClientInterface
	httpClient changelog.HTTPClient
}

type ecosystem struct {
	name          string
	provider      dependency.Provider
	install       func(ctx context.Context, dir string) error // install deps before scanning
	listUpdates   func(ctx context.Context, dir string) ([]dependency.UpdateInfo, error)
	scanVulns     func(ctx context.Context, dir string) ([]dependency.AdvisoryInfo, error)
	applyPatches  func(ctx context.Context, dir string, deps []dependency.Dependency) ([]dependency.Dependency, error)
	applyUpdates  func(ctx context.Context, dir string, deps []dependency.Dependency) ([]dependency.Dependency, error)
	manifestFiles []string // files to commit after patching
}

func main() {
	if err := run(); err != nil {
		log.Fatalf("error: %v", err)
	}
}

func run() error {
	httpClient := &http.Client{Timeout: 30 * time.Second}
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}
	d := &deps{
		runner:     exec.NewDefaultRunner(),
		ghClient:   gh.NewClient(httpClient, cfg.Token),
		httpClient: httpClient,
	}
	return runWithDeps(cfg, d)
}

func detectEcosystems(projectDir string, runner exec.CommandRunner, includeDev bool) []ecosystem {
	var ecosystems []ecosystem

	if fileExists(filepath.Join(projectDir, "package.json")) {
		npmScanner := scanner.New(runner, includeDev)
		npmSecurity := security.NewNpmAuditScanner(runner, includeDev)
		npmUpdater := updater.New(runner)
		ecosystems = append(ecosystems, ecosystem{
			name:     "npm",
			provider: dependency.NewPackageJSONProvider(),
			install: func(ctx context.Context, dir string) error {
				_, err := runner.Run(ctx, dir, "npm", "install", "--ignore-scripts", "--no-audit")
				return err
			},
			listUpdates: func(ctx context.Context, dir string) ([]dependency.UpdateInfo, error) {
				updates, err := npmScanner.ListAvailable(ctx, dir)
				if err != nil {
					return nil, err
				}
				var infos []dependency.UpdateInfo
				for _, u := range updates {
					infos = append(infos, dependency.UpdateInfo{
						Name: u.Name, Current: u.Current, Wanted: u.Wanted,
						Latest: u.Latest, UpdateType: u.UpdateType,
					})
				}
				return infos, nil
			},
			scanVulns: func(ctx context.Context, dir string) ([]dependency.AdvisoryInfo, error) {
				advs, err := npmSecurity.Scan(ctx, dir)
				if err != nil {
					return nil, err
				}
				var infos []dependency.AdvisoryInfo
				for _, a := range advs {
					infos = append(infos, dependency.AdvisoryInfo{
						ID: a.ID, Package: a.Package, Severity: a.Severity,
						Title: a.Title, AffectedVersions: a.AffectedVersions,
						FixedVersion: a.FixedVersion, URL: a.URL,
					})
				}
				return infos, nil
			},
			applyPatches:  npmUpdater.ApplyPatches,
			applyUpdates:  npmUpdater.ApplyUpdates,
			manifestFiles: []string{"package.json", "package-lock.json"},
		})
	}

	if fileExists(filepath.Join(projectDir, "composer.json")) {
		composerScanner := composer.NewScanner(runner, includeDev)
		composerSecurity := composer.NewAuditScanner(runner, includeDev)
		composerUpdater := composer.NewUpdater(runner)
		ecosystems = append(ecosystems, ecosystem{
			name:     "composer",
			provider: composer.NewComposerProvider(),
			install: func(ctx context.Context, dir string) error {
				_, err := runner.Run(ctx, dir, "composer", "install", "--no-scripts", "--no-interaction", "--ignore-platform-reqs")
				return err
			},
			listUpdates:   composerScanner.ListAvailable,
			scanVulns:     composerSecurity.Scan,
			applyPatches:  composerUpdater.ApplyPatches,
			applyUpdates:  composerUpdater.ApplyUpdates,
			manifestFiles: []string{"composer.json", "composer.lock"},
		})
	}

	if fileExists(filepath.Join(projectDir, "go.mod")) {
		goScanner := gomod.NewScanner(runner)
		goSecurity := gomod.NewVulnScanner(runner)
		goUpdater := gomod.NewUpdater(runner)
		ecosystems = append(ecosystems, ecosystem{
			name:     "gomod",
			provider: gomod.NewProvider(),
			install: func(ctx context.Context, dir string) error {
				_, err := runner.Run(ctx, dir, "go", "mod", "download")
				return err
			},
			listUpdates:   goScanner.ListAvailable,
			scanVulns:     goSecurity.Scan,
			applyPatches:  goUpdater.ApplyPatches,
			applyUpdates:  goUpdater.ApplyUpdates,
			manifestFiles: []string{"go.mod", "go.sum"},
		})
	}

	return ecosystems
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func runWithDeps(cfg *config.Config, d *deps) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	log.Printf("loaded config for %s/%s", cfg.Owner, cfg.Repo)

	// Resolve base branch
	baseBranch := cfg.BaseBranch
	var err error
	if baseBranch == "" {
		baseBranch, err = d.ghClient.GetDefaultBranch(ctx, cfg.Owner, cfg.Repo)
		if err != nil {
			return fmt.Errorf("getting default branch: %w", err)
		}
	}
	log.Printf("base branch: %s", baseBranch)

	// Detect ecosystems
	ecosystems := detectEcosystems(cfg.ProjectDir, d.runner, cfg.IncludeDev)
	if len(ecosystems) == 0 {
		log.Println("no supported ecosystems detected, nothing to do")
		setOutput("patches_applied", "0")
		setOutput("updates_available", "0")
		setOutput("vulnerabilities_found", "0")
		return nil
	}
	log.Printf("detected ecosystems: %s", ecosystemNames(ecosystems))

	// Aggregate across all ecosystems
	var allDeps []dependency.Dependency
	var allAdvisories []dependency.AdvisoryInfo
	type ecosystemPatches struct {
		eco     ecosystem
		patches []dependency.Dependency
	}
	var patchGroups []ecosystemPatches
	var updateGroups []ecosystemPatches

	for _, eco := range ecosystems {
		log.Printf("[%s] discovering dependencies", eco.name)
		discovered, err := eco.provider.Discover(cfg.ProjectDir)
		if err != nil {
			return fmt.Errorf("[%s] discovering dependencies: %w", eco.name, err)
		}
		log.Printf("[%s] found %d dependencies", eco.name, len(discovered))

		if len(discovered) == 0 {
			continue
		}

		log.Printf("[%s] installing dependencies", eco.name)
		if err := eco.install(ctx, cfg.ProjectDir); err != nil {
			return fmt.Errorf("[%s] installing dependencies: %w", eco.name, err)
		}

		updates, err := eco.listUpdates(ctx, cfg.ProjectDir)
		if err != nil {
			return fmt.Errorf("[%s] listing updates: %w", eco.name, err)
		}
		log.Printf("[%s] found %d available updates", eco.name, len(updates))

		advisories, err := eco.scanVulns(ctx, cfg.ProjectDir)
		if err != nil {
			log.Printf("[%s] warning: vulnerability scan failed: %v", eco.name, err)
		} else {
			log.Printf("[%s] found %d advisories", eco.name, len(advisories))
		}

		enriched := dependency.Enrich(discovered, updates, advisories)
		allDeps = append(allDeps, enriched...)
		allAdvisories = append(allAdvisories, advisories...)

		// Collect patches per ecosystem (need separate updaters)
		var patches []dependency.Dependency
		for _, dep := range enriched {
			if dep.UpdateType == "patch" {
				patches = append(patches, dep)
			}
		}
		if len(patches) > 0 {
			patchGroups = append(patchGroups, ecosystemPatches{eco: eco, patches: patches})
		}

		var minorMajorDeps []dependency.Dependency
		for _, dep := range enriched {
			if dep.UpdateType == "minor" || dep.UpdateType == "major" {
				minorMajorDeps = append(minorMajorDeps, dep)
			}
		}
		if len(minorMajorDeps) > 0 {
			updateGroups = append(updateGroups, ecosystemPatches{eco: eco, patches: minorMajorDeps})
		}
	}

	if len(allDeps) == 0 {
		log.Println("no dependencies found, nothing to do")
		setOutput("patches_applied", "0")
		setOutput("updates_available", "0")
		setOutput("vulnerabilities_found", "0")
		return nil
	}

	// Classify all
	var minorMajor []dependency.Dependency
	for _, dep := range allDeps {
		if dep.UpdateType == "minor" || dep.UpdateType == "major" {
			minorMajor = append(minorMajor, dep)
		}
	}

	patchesApplied := 0
	var prURL string
	baseRef := "heads/" + baseBranch

	// Phase 1: auto-merge patch updates via a dedicated branch + immediate squash merge.
	if cfg.AutoPatch && len(patchGroups) > 0 {
		patchBranch := cfg.PatchBranch()
		baseSHA, err := d.ghClient.GetRef(ctx, cfg.Owner, cfg.Repo, baseRef)
		if err != nil {
			return fmt.Errorf("getting base ref: %w", err)
		}
		if err := ensureBranch(ctx, d, cfg, patchBranch, baseSHA); err != nil {
			return err
		}

		var appliedPatchDeps []dependency.Dependency
		committedPatches := false
		for _, pg := range patchGroups {
			log.Printf("[%s] applying %d patch updates", pg.eco.name, len(pg.patches))
			applied, applyErr := pg.eco.applyPatches(ctx, cfg.ProjectDir, pg.patches)
			if applyErr != nil {
				log.Printf("[%s] warning: some patches failed: %v", pg.eco.name, applyErr)
			}
			if len(applied) > 0 {
				files, err := readManifests(cfg.ProjectDir, pg.eco.manifestFiles)
				if err != nil {
					return fmt.Errorf("[%s] reading manifests after patch updates: %w", pg.eco.name, err)
				}
				msg := fmt.Sprintf("chore(deps): apply %s patch updates", pg.eco.name)
				if _, err := d.ghClient.CommitFiles(ctx, cfg.Owner, cfg.Repo, patchBranch, msg, files); err != nil {
					return fmt.Errorf("[%s] committing patch updates: %w", pg.eco.name, err)
				}
				patchesApplied += len(applied)
				appliedPatchDeps = append(appliedPatchDeps, applied...)
				committedPatches = true
				log.Printf("[%s] committed %d patch updates", pg.eco.name, len(applied))
			}
		}

		if committedPatches {
			title := fmt.Sprintf("chore(deps): %s patch updates", cfg.ScheduleLabel)
			body := reporting.GeneratePRBody(appliedPatchDeps, map[string]*changelog.ChangelogInfo{})
			prNumber, found, err := d.ghClient.FindOpenPR(ctx, cfg.Owner, cfg.Repo, patchBranch, baseBranch)
			if err != nil {
				return fmt.Errorf("finding existing patch PR: %w", err)
			}
			if !found {
				_, num, err := d.ghClient.CreatePR(ctx, cfg.Owner, cfg.Repo, gh.PRRequest{
					Title: title,
					Body:  body,
					Head:  patchBranch,
					Base:  baseBranch,
				})
				if err != nil {
					return fmt.Errorf("creating patch PR: %w", err)
				}
				prNumber = num
				log.Printf("created patch PR #%d", prNumber)
			}
			if err := d.ghClient.MergePR(ctx, cfg.Owner, cfg.Repo, prNumber, "squash"); err != nil {
				log.Printf("warning: could not auto-merge patch PR #%d (leaving it open for manual merge): %v", prNumber, err)
			} else {
				log.Printf("auto-merged patch PR #%d", prNumber)
			}
		}
	}

	// Phase 2: minor/major updates via a review PR, cut from the now-updated base.
	committedToBranch := false
	if cfg.CreatePR && len(minorMajor) > 0 {
		updateBranch := cfg.UpdateBranch()
		baseSHA, err := d.ghClient.GetRef(ctx, cfg.Owner, cfg.Repo, baseRef)
		if err != nil {
			return fmt.Errorf("getting base ref: %w", err)
		}
		if err := ensureBranch(ctx, d, cfg, updateBranch, baseSHA); err != nil {
			return err
		}

		for _, ug := range updateGroups {
			log.Printf("[%s] applying %d minor/major updates", ug.eco.name, len(ug.patches))
			applied, applyErr := ug.eco.applyUpdates(ctx, cfg.ProjectDir, ug.patches)
			if applyErr != nil {
				log.Printf("[%s] warning: some updates failed: %v", ug.eco.name, applyErr)
			}
			if len(applied) > 0 {
				files, err := readManifests(cfg.ProjectDir, ug.eco.manifestFiles)
				if err != nil {
					return fmt.Errorf("[%s] reading manifests after minor/major updates: %w", ug.eco.name, err)
				}
				msg := fmt.Sprintf("chore(deps): apply %s minor/major updates", ug.eco.name)
				if _, err := d.ghClient.CommitFiles(ctx, cfg.Owner, cfg.Repo, updateBranch, msg, files); err != nil {
					return fmt.Errorf("[%s] committing minor/major updates: %w", ug.eco.name, err)
				}
				committedToBranch = true
				log.Printf("[%s] committed %d minor/major updates", ug.eco.name, len(applied))
			}
		}

		if !committedToBranch {
			log.Printf("no changes committed to branch %s, skipping PR creation", updateBranch)
		} else {
			clProvider := changelog.NewNpmRegistryProvider(d.httpClient)
			changelogs := make(map[string]*changelog.ChangelogInfo)
			for _, dep := range allDeps {
				if dep.UpdateType == "major" && isNpmPackage(dep.Name) {
					cl, err := clProvider.FetchChangelog(ctx, dep.Name, dep.CurrentVersion, dep.LatestVersion)
					if err != nil {
						log.Printf("warning: changelog fetch failed for %s: %v", dep.Name, err)
						continue
					}
					changelogs[dep.Name] = cl
				}
			}

			prBody := reporting.GeneratePRBody(allDeps, changelogs)
			title := fmt.Sprintf("chore(deps): %s dependency updates", cfg.ScheduleLabel)

			prNumber, found, err := d.ghClient.FindOpenPR(ctx, cfg.Owner, cfg.Repo, updateBranch, baseBranch)
			if err != nil {
				return fmt.Errorf("finding existing PR: %w", err)
			}

			pr := gh.PRRequest{
				Title: title,
				Body:  prBody,
				Head:  updateBranch,
				Base:  baseBranch,
			}

			if found {
				if err := d.ghClient.UpdatePR(ctx, cfg.Owner, cfg.Repo, prNumber, pr); err != nil {
					return fmt.Errorf("updating PR: %w", err)
				}
				prURL = fmt.Sprintf("https://github.com/%s/%s/pull/%d", cfg.Owner, cfg.Repo, prNumber)
				log.Printf("updated PR #%d", prNumber)
			} else {
				url, num, err := d.ghClient.CreatePR(ctx, cfg.Owner, cfg.Repo, pr)
				if err != nil {
					return fmt.Errorf("creating PR: %w", err)
				}
				prURL = url
				log.Printf("created PR #%d: %s", num, url)

				if len(cfg.Labels) > 0 {
					if err := d.ghClient.AddLabels(ctx, cfg.Owner, cfg.Repo, num, cfg.Labels); err != nil {
						log.Printf("warning: failed to add labels: %v", err)
					}
				}
			}
		}
	}

	// Write outputs
	setOutput("patches_applied", fmt.Sprintf("%d", patchesApplied))
	setOutput("updates_available", fmt.Sprintf("%d", len(minorMajor)))
	setOutput("vulnerabilities_found", fmt.Sprintf("%d", len(allAdvisories)))
	if prURL != "" {
		setOutput("pr_url", prURL)
	}

	summary := reporting.GenerateSummary(allDeps, patchesApplied)
	writeSummary(summary)

	log.Println("dependency curator completed successfully")
	return nil
}

func ensureBranch(ctx context.Context, d *deps, cfg *config.Config, branch, baseSHA string) error {
	if err := d.ghClient.CreateBranch(ctx, cfg.Owner, cfg.Repo, branch, baseSHA); err != nil {
		exists, existsErr := d.ghClient.BranchExists(ctx, cfg.Owner, cfg.Repo, branch)
		if existsErr != nil || !exists {
			return fmt.Errorf("creating branch %s: %w", branch, err)
		}
		if err := d.ghClient.UpdateRef(ctx, cfg.Owner, cfg.Repo, "heads/"+branch, baseSHA); err != nil {
			return fmt.Errorf("updating branch ref %s: %w", branch, err)
		}
		log.Printf("reset branch %s to %s", branch, baseSHA[:7])
	} else {
		log.Printf("created branch %s", branch)
	}
	return nil
}

func readManifests(dir string, names []string) (map[string][]byte, error) {
	files := make(map[string][]byte)
	for _, fname := range names {
		data, err := os.ReadFile(filepath.Join(dir, fname))
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", fname, err)
		}
		files[fname] = data
	}
	return files, nil
}

func isNpmPackage(name string) bool {
	return !strings.Contains(name, "/") || strings.HasPrefix(name, "@")
}

func ecosystemNames(ecosystems []ecosystem) string {
	names := make([]string, len(ecosystems))
	for i, e := range ecosystems {
		names[i] = e.name
	}
	return strings.Join(names, ", ")
}

func setOutput(name, value string) {
	outputFile := os.Getenv("GITHUB_OUTPUT")
	if outputFile == "" {
		log.Printf("warning: GITHUB_OUTPUT not set, output %s=%s not written", name, value)
		return
	}
	f, err := os.OpenFile(outputFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Printf("warning: could not write output %s: %v", name, err)
		return
	}
	defer f.Close()
	// Use delimiter for multi-line safety
	delimiter := "EOF_DEPENDENCY_CURATOR"
	fmt.Fprintf(f, "%s<<%s\n%s\n%s\n", name, delimiter, value, delimiter)
}

func writeSummary(content string) {
	summaryFile := os.Getenv("GITHUB_STEP_SUMMARY")
	if summaryFile == "" {
		return
	}
	f, err := os.OpenFile(summaryFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Printf("warning: could not write summary: %v", err)
		return
	}
	defer f.Close()
	fmt.Fprint(f, content)
}
