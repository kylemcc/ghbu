package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/genuinetools/pkg/cli"
	"github.com/google/go-github/github"

	"golang.org/x/oauth2"
)

const (
	// MaxRetries is the max number of attempts for github api operations
	MaxRetries = 2
)

var (
	orgName  string
	userName string
	dir      string
	token    string
	replace  bool
)

// Config holds the configuration for the backup procedure
type Config struct {
	// The directory to which the specified account should be backed up
	Dir string

	// If true, existing directories will be deleted and replaced
	Replace bool

	// If true, back up authenticated user's repos
	Self bool

	Client *github.Client
}

func logError(fs string, v ...interface{}) {
	fmt.Fprintf(os.Stderr, fs, v...)
}

func main() {
	p := cli.NewProgram()
	p.Name = "ghbu"
	p.Description = "A tool for backing up all Github repos for a user/organization"
	p.FlagSet = flag.NewFlagSet("global", flag.ExitOnError)

	p.FlagSet.StringVar(&orgName, "org", "", "Github Organization to backup. Takes precedence over -user")
	p.FlagSet.StringVar(&userName, "user", "", "Github user to backup (if not specificed, the authenticated user is assumed).")
	p.FlagSet.StringVar(&dir, "dir", "", "Directory where repositories should be backed up (required)")
	p.FlagSet.StringVar(&token, "token", "", "Github auth token. Will use the value of $GITHUB_TOKEN if set (required)")
	p.FlagSet.BoolVar(&replace, "replace", false, "Replace existing repositories in -dir instead of attempting to update")

	p.Before = func(ctx context.Context) error {
		if t := os.Getenv("GITHUB_TOKEN"); t != "" {
			token = t
		}

		if dir == "" || token == "" {
			p.FlagSet.Usage()
			return fmt.Errorf("error: directory and token are required")
		}

		if _, err := os.Stat(dir); os.IsNotExist(err) {
			return fmt.Errorf("error: directory does not exist: %s", dir)
		}
		return nil
	}

	p.Action = func(ctx context.Context, args []string) error {
		start := time.Now()
		ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
		tc := oauth2.NewClient(ctx, ts)
		gc := github.NewClient(tc)

		cfg := &Config{
			Dir:     dir,
			Replace: replace,
			Client:  gc,
		}

		if orgName != "" {
			o, err := getOrg(ctx, gc, orgName)
			if err != nil {
				logError("error: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("Backing up organization %s...\n", o.GetLogin())
			backupOrg(ctx, orgName, cfg)
		} else {
			u, err := getUser(ctx, gc, userName)
			if err != nil {
				logError("error: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("Backing up user %s...\n", u.GetLogin())
			backupUser(ctx, userName, cfg)
		}

		fmt.Printf("Backup finished. Took %v\n", time.Since(start))

		return nil
	}

	p.Run()
}

func getUser(ctx context.Context, gc *github.Client, user string) (*github.User, error) {
	u, _, err := gc.Users.Get(ctx, user)
	return u, err
}

func getOrg(ctx context.Context, gc *github.Client, org string) (*github.Organization, error) {
	o, _, err := gc.Organizations.Get(ctx, org)
	return o, err
}

func backupOrg(ctx context.Context, org string, cfg *Config) error {
	repos, err := getOrgRepos(ctx, cfg.Client, org)
	if err != nil {
		return err
	}

	fmt.Printf("Backing up %d repositories to %s...\n", len(repos), cfg.Dir)
	for _, r := range repos {
		if err := cloneRepo(ctx, r, cfg); err != nil {
			logError("error backing up repository %s: %v\n", r.GetFullName(), err)
		}
	}
	return nil
}

func getOrgRepos(ctx context.Context, gc *github.Client, org string) ([]*github.Repository, error) {
	opt := &github.RepositoryListByOrgOptions{
		ListOptions: github.ListOptions{PerPage: 25},
	}

	var allRepos []*github.Repository
	retry := 0
	for {
		repos, resp, err := gc.Repositories.ListByOrg(ctx, org, opt)
		if err != nil && retry < MaxRetries {
			retry++
			continue
		} else if err != nil {
			return nil, err
		}

		allRepos = append(allRepos, repos...)
		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
		retry = 0
	}
	return allRepos, nil
}

func backupUser(ctx context.Context, user string, cfg *Config) error {
	repos, err := getUserRepos(ctx, cfg.Client, user)
	if err != nil {
		return err
	}

	fmt.Printf("Backing up %d repositories to %s...\n", len(repos), cfg.Dir)
	for _, r := range repos {
		if err := cloneRepo(ctx, r, cfg); err != nil {
			logError("error backing up repository %s: %v\n", r.GetFullName(), err)
		}
	}
	return nil
}

func getUserRepos(ctx context.Context, gc *github.Client, user string) ([]*github.Repository, error) {
	opt := &github.RepositoryListOptions{
		Affiliation: "owner",
		ListOptions: github.ListOptions{PerPage: 25},
	}

	var allRepos []*github.Repository
	retry := 0
	for {
		repos, resp, err := gc.Repositories.List(ctx, user, opt)
		if err != nil && retry < MaxRetries {
			retry++
			continue
		} else if err != nil {
			return nil, err
		}

		allRepos = append(allRepos, repos...)
		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
		retry = 0
	}
	fmt.Printf("r: %+v\n\n", allRepos[0])
	return allRepos, nil
}

func cloneRepo(ctx context.Context, r *github.Repository, cfg *Config) error {
	dest := filepath.Join(cfg.Dir, r.GetName())
	if _, err := os.Stat(dest); err == nil {
		// dir exists
		if cfg.Replace {
			if err := os.RemoveAll(dest); err != nil {
				return err
			}
			// With dest removed, will proceed to clone below
		} else {
			return updateRepo(ctx, r, cfg)
		}
	} else if !os.IsNotExist(err) {
		// some non-does-not-exist error
		return err
	}

	var cloneURL string
	if s := r.GetSSHURL(); s != "" {
		cloneURL = s
	} else {
		cloneURL = r.GetCloneURL()
	}

	fmt.Printf("Backing up %v...\n", r.GetFullName())
	cmd := exec.Command("git", "clone", cloneURL)
	cmd.Dir = cfg.Dir
	err := cmd.Run()
	if err != nil {
		logError("error cloning %v: %v\n", r.GetFullName(), err)
		return err
	}
	fmt.Printf("Done.\n")
	return nil
}

func updateRepo(ctx context.Context, r *github.Repository, cfg *Config) error {
	fmt.Printf("Updating %v...\n", r.GetFullName())
	cmd := exec.Command("git", "pull")
	cmd.Dir = filepath.Join(cfg.Dir, r.GetName())
	err := cmd.Run()
	if err != nil {
		logError("error updating %v: %v\n", r.GetFullName(), err)
		return err
	}
	fmt.Printf("Done.\n")
	return nil
}
