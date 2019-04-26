package prs

import "github.com/google/go-github/v25/github"

type PRWithReviews struct {
	PR      github.Issue
	Reviews []*github.PullRequestReview
}
