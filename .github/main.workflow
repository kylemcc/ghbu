workflow "Build Docker Image" {
  on = "push"
  resolves = ["Send Slack Notification"]
}

action "Check Master Branch" {
  uses = "actions/bin/filter@b2bea07"
  args = "branch master"
}

action "Docker Login" {
  uses = "actions/docker/login@76ff57a"
  needs = ["Check Master Branch"]
  secrets = ["DOCKER_USERNAME", "DOCKER_PASSWORD"]
}

action "Build Image" {
  uses = "actions/docker/cli@76ff57a"
  needs = ["Docker Login"]
  args = "build -t kylemcc/prv:ghbu ."
}

action "Push Image" {
  uses = "actions/docker/cli@76ff57a"
  needs = ["Build Image"]
  args = "push kylemcc/prv:ghbu"
}

action "Send Slack Notification" {
  uses = "kylemcc/actions/slack-webhook@master"
  needs = ["Push Image"]
  secrets = ["SLACK_WEBHOOK_URL"]
  env = {
    SLACK_MESSAGE = "$GITHUB_REPOSITORY: Build Complete"
  }
}
