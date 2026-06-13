package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/JBSommeling/dependency-curator/internal/changelog"
	"github.com/JBSommeling/dependency-curator/internal/config"
	"github.com/JBSommeling/dependency-curator/internal/dependency"
	"github.com/JBSommeling/dependency-curator/internal/exec"
	gh "github.com/JBSommeling/dependency-curator/internal/github"
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

	// Step 2: Discover dependencies
	provider := dependency.NewPackageJSONProvider()
	deps, err := provider.Discover(cfg.ProjectDir)
	if err != nil {
		return fmt.Errorf("discovering dependencies: %w", err)
	}

	if !cfg.IncludeDev {
		var filtered []dependency.Dependency
		for _, dep := range deps {
			if !dep.IsDev {
				filtered = append(filtered, dep)
			}
		}
		deps = filtered
	}
	log.Printf("discovered %d dependencies", len(deps))

	if len(deps) == 0 {
		log.Println("no dependencies found, nothing to do")
		setOutput("patches_applied", "0")
		setOutput("updates_available", "0")
		setOutput("vulnerabilities_found", "0")
		return nil
	}

	// Step 3: Check for available updates
	sc := scanner.New(d.runner)
	updates, err := sc.ListAvailable(ctx, cfg.ProjectDir)
	if err != nil {
		return fmt.Errorf("listing available updates: %w", err)
	}
	log.Printf("found %d available updates", len(updates))

	// Step 4: Scan vulnerabilities
	secScanner := security.NewNpmAuditScanner(d.runner)
	advisories, err := secScanner.Scan(ctx, cfg.ProjectDir)
	if err != nil {
		log.Printf("warning: vulnerability scan failed: %v", err)
		// Non-fatal — continue without advisory data
	}
	log.Printf("found %d advisories", len(advisories))

	// Step 5: Enrich dependencies
	enriched := dependency.Enrich(deps, updates, advisories)

	// Classify
	var patches, minorMajor []dependency.Dependency
	for _, dep := range enriched {
		switch dep.UpdateType {
		case "patch":
			patches = append(patches, dep)
		case "minor", "major":
			minorMajor = append(minorMajor, dep)
		}
	}

	patchesApplied := 0
	updateBranch := cfg.UpdateBranch()

	// Step 6: Ensure update branch exists
	needsBranch := (cfg.AutoPatch && len(patches) > 0) || (cfg.CreatePR && len(minorMajor) > 0)
	if needsBranch {
		exists, err := d.ghClient.BranchExists(ctx, cfg.Owner, cfg.Repo, updateBranch)
		if err != nil {
			return fmt.Errorf("checking branch: %w", err)
		}
		if !exists {
			baseSHA, err := d.ghClient.GetRef(ctx, cfg.Owner, cfg.Repo, "heads/"+baseBranch)
			if err != nil {
				return fmt.Errorf("getting base ref: %w", err)
			}
			if err := d.ghClient.CreateBranch(ctx, cfg.Owner, cfg.Repo, updateBranch, baseSHA); err != nil {
				return fmt.Errorf("creating update branch: %w", err)
			}
			log.Printf("created branch %s", updateBranch)
		}
	}

	// Step 7: Apply patch updates
	if cfg.AutoPatch && len(patches) > 0 {
		log.Printf("applying %d patch updates", len(patches))
		upd := updater.New(d.runner)
		applied, err := upd.ApplyPatches(ctx, cfg.ProjectDir, enriched)
		if err != nil {
			log.Printf("warning: some patches failed: %v", err)
		}
		patchesApplied = len(applied)

		if patchesApplied > 0 {
			// Read updated files and commit via API
			files := make(map[string][]byte)
			for _, fname := range []string{"package.json", "package-lock.json"} {
				data, err := os.ReadFile(fmt.Sprintf("%s/%s", cfg.ProjectDir, fname))
				if err != nil {
					continue
				}
				files[fname] = data
			}
			if len(files) > 0 {
				_, err := d.ghClient.CommitFiles(ctx, cfg.Owner, cfg.Repo, updateBranch,
					"chore(deps): apply weekly patch updates", files)
				if err != nil {
					return fmt.Errorf("committing patch updates: %w", err)
				}
				log.Printf("committed %d patch updates", patchesApplied)
			}
		}
	}

	// Step 8: Create/update PR for minor+major
	var prURL string
	if cfg.CreatePR && len(minorMajor) > 0 {
		// Fetch changelogs for major updates
		clProvider := changelog.NewNpmRegistryProvider(d.httpClient)
		changelogs := make(map[string]*changelog.ChangelogInfo)
		for _, dep := range enriched {
			if dep.UpdateType == "major" {
				cl, err := clProvider.FetchChangelog(dep.Name, dep.CurrentVersion, dep.LatestVersion)
				if err != nil {
					log.Printf("warning: changelog fetch failed for %s: %v", dep.Name, err)
					continue
				}
				changelogs[dep.Name] = cl
			}
		}

		// Generate report
		prBody := reporting.GeneratePRBody(enriched, changelogs)

		title := fmt.Sprintf("chore(deps): %s dependency updates", cfg.ScheduleLabel)

		// Check for existing PR
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

	// Step 9: Write outputs
	setOutput("patches_applied", fmt.Sprintf("%d", patchesApplied))
	setOutput("updates_available", fmt.Sprintf("%d", len(minorMajor)))
	setOutput("vulnerabilities_found", fmt.Sprintf("%d", len(advisories)))
	if prURL != "" {
		setOutput("pr_url", prURL)
	}

	// Write step summary
	summary := reporting.GenerateSummary(enriched, patchesApplied)
	writeSummary(summary)

	log.Println("dependency guardian completed successfully")
	return nil
}

func setOutput(name, value string) {
	outputFile := os.Getenv("GITHUB_OUTPUT")
	if outputFile == "" {
		fmt.Printf("::set-output name=%s::%s\n", name, value)
		return
	}
	f, err := os.OpenFile(outputFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Printf("warning: could not write output %s: %v", name, err)
		return
	}
	defer f.Close()
	// Use delimiter for multi-line safety
	delimiter := "EOF_DEPENDENCY_GUARDIAN"
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
