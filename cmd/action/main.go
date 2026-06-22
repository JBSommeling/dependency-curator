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
	updateBranch := cfg.UpdateBranch()

	// Ensure update branch exists
	needsBranch := (cfg.AutoPatch && len(patchGroups) > 0) || (cfg.CreatePR && len(minorMajor) > 0)
	if needsBranch {
		baseSHA, err := d.ghClient.GetRef(ctx, cfg.Owner, cfg.Repo, "heads/"+baseBranch)
		if err != nil {
			return fmt.Errorf("getting base ref: %w", err)
		}
		if err := d.ghClient.CreateBranch(ctx, cfg.Owner, cfg.Repo, updateBranch, baseSHA); err != nil {
			exists, existsErr := d.ghClient.BranchExists(ctx, cfg.Owner, cfg.Repo, updateBranch)
			if existsErr != nil || !exists {
				return fmt.Errorf("creating update branch: %w", err)
			}
			if err := d.ghClient.UpdateRef(ctx, cfg.Owner, cfg.Repo, "heads/"+updateBranch, baseSHA); err != nil {
				return fmt.Errorf("updating branch ref: %w", err)
			}
			log.Printf("updated branch %s to %s", updateBranch, baseSHA[:7])
		} else {
			log.Printf("created branch %s", updateBranch)
		}
	}

	// Apply patches per ecosystem
	if cfg.AutoPatch {
		for _, pg := range patchGroups {
			log.Printf("[%s] applying %d patch updates", pg.eco.name, len(pg.patches))
			applied, applyErr := pg.eco.applyPatches(ctx, cfg.ProjectDir, pg.patches)
			if applyErr != nil {
				log.Printf("[%s] warning: some patches failed: %v", pg.eco.name, applyErr)
			}
			count := len(applied)
			if count > 0 {
				files := make(map[string][]byte)
				for _, fname := range pg.eco.manifestFiles {
					path := filepath.Join(cfg.ProjectDir, fname)
					data, err := os.ReadFile(path)
					if err != nil {
						return fmt.Errorf("[%s] reading %s after patch updates: %w", pg.eco.name, fname, err)
					}
					files[fname] = data
				}
				msg := fmt.Sprintf("chore(deps): apply %s patch updates", pg.eco.name)
				_, err := d.ghClient.CommitFiles(ctx, cfg.Owner, cfg.Repo, updateBranch, msg, files)
				if err != nil {
					return fmt.Errorf("[%s] committing patch updates: %w", pg.eco.name, err)
				}
				patchesApplied += count
				log.Printf("[%s] committed %d patch updates", pg.eco.name, count)
			}
		}
	}

	// Create/update PR for minor+major
	var prURL string
	if cfg.CreatePR && len(minorMajor) > 0 {
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
