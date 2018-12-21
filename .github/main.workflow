workflow "Build Docker Image" {
  on = "push"
  resolves = ["kylemcc/actions/slack-webhook@master"]
}

action "master-branch" {
  uses = "actions/bin/filter@b2bea07"
  args = "branch master"
}

action "docker-login" {
  uses = "actions/docker/login@76ff57a"
  needs = ["master-branch"]
  secrets = ["DOCKER_USERNAME", "DOCKER_PASSWORD"]
}

action "build-image" {
  uses = "actions/docker/cli@76ff57a"
  needs = ["docker-login"]
  args = "build -t kylemcc/prv:ghbu ."
}

action "push-image" {
  uses = "actions/docker/cli@76ff57a"
  needs = ["build-image"]
  args = "push kylemcc/prv:ghbu"
}

action "kylemcc/actions/slack-webhook@master" {
  uses = "kylemcc/actions/slack-webhook@master"
  needs = ["push-image"]
  secrets = ["SLACK_WEBHOOK_URL"]
  env = {
    SLACK_MESSAGE = "$GITHUB_REPOSITORY: Build Complete"
  }
}
