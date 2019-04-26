# PRs - list all your open PRs and those that you have to act upon, i.e. review)

Attention, please: I hacked this tool together in a rush with the aim of getting
output as quickly as possible. That's why all the goroutine handling is a bit
ugly. The GitHub API doesn't support batch requests so the tool has to fire up
quite some HTTP requests. To speed things up there's a goroutine created for
every request to make the work as concurrent as possible.

## Getting started

1. Clone this repo (be sure to have a Go dev environment) and `go install`.
2. Execute `prs` and follow the instructions.

## What you will get

When you have configured `prs` correctly after calling it for the first time
you'll be presented with a table that lists all your PRs first together with the
usernames of the teammates that have already approved them.

Below those the table lists all PRs of your teammates in the `dcos` or
`mesosphere` org that you haven't reviewed, yet. PRs from the
`mesosphere/dcos-enterprise` that mirror `dcos/dcos` PRs are also listed there
so you don't have to check manually.
