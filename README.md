## ghbu

A tool for downloading all Github repos for a user/organization

```
ghbu -  A tool for backing up all Github repos for a user/organization.

Usage: ghbu <command>

Flags:

  -d, --dir       Directory where repositories should be backed up (required) (default: <none>)
  -o, --org       Github Organization to backup. Takes precedence over -user (default: <none>)
  -p, --parallel  Number of repositories to clone in parallel (default: 2)
  -r, --replace   Replace existing repositories in -dir instead of attempting to update (default: false)
  -t, --token     Github auth token. Will use the value of $GITHUB_TOKEN if set (required) (default: <none>)
  -u, --user      Github user to backup (if not specificed, the authenticated user is assumed). (default: <none>)

Commands:

  version  Show the version information.

```
