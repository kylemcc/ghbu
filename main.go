package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/google/go-github/github"

	"golang.org/x/oauth2"
)

const (
	MaxRetries = 2
)

var (
	orgName  = flag.String("org", "", "Github Organization to backup. Takes precedence over -user")
	userName = flag.String("user", "", "Github user to backup (if not specificed, the authenticated user is assumed).")
	dir      = flag.String("dir", "", "Directory where repositories should be backed up (required)")
	token    = flag.String("token", "", "Github auth token (required)")
	replace  = flag.Bool("replace", false, "Replace existing repositories in -dir instead of attempting to update")
)

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
	flag.Parse()

	if *dir == "" || *token == "" {
		flag.Usage()
		os.Exit(1)
	}

	if _, err := os.Stat(*dir); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "error: directory does not exist: %s\n", *dir)
		os.Exit(1)
	}

	ctx := context.Background()
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: *token})
	tc := oauth2.NewClient(ctx, ts)
	gc := github.NewClient(tc)

	cfg := &Config{
		Dir:     *dir,
		Replace: *replace,
		Client:  gc,
	}

	if *orgName != "" {
		o, err := getOrg(ctx, gc, *orgName)
		if err != nil {
			logError("error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Backing up organization %s...\n", o.GetLogin())
		backupOrg(ctx, *orgName, cfg)
	} else {
		u, err := getUser(ctx, gc, *userName)
		if err != nil {
			logError("error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Backing up user %s...\n", u.GetLogin())
		backupUser(ctx, *userName, cfg)
	}
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

	var cloneUrl string
	if s := r.GetSSHURL(); s != "" {
		cloneUrl = s
	} else {
		cloneUrl = r.GetCloneURL()
	}

	fmt.Printf("Backing up %v...\n", r.GetFullName())
	cmd := exec.Command("git", "clone", cloneUrl)
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
