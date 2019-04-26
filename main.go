package main

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/makkes/prs/table"

	"github.com/google/go-github/v25/github"
	"github.com/makkes/prs/config"
	"github.com/makkes/prs/prs"
	"golang.org/x/oauth2"
)

func getOwnerAndRepoAndID(url string) (string, string, string) {
	res := regexp.MustCompile("([^/]*)/([^/]*)/(pull|issues)/([^/]*)").FindStringSubmatch(url)
	if len(res) < 5 {
		return "", "", ""
	}
	return res[1], res[2], res[4]
}

type bySubmitDate []*github.PullRequestReview

func (s bySubmitDate) Len() int {
	return len(s)
}
func (s bySubmitDate) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}
func (s bySubmitDate) Less(i, j int) bool {
	return s[i].GetSubmittedAt().After(s[j].GetSubmittedAt())
}

type byCreationDate []prs.PRWithReviews

func (s byCreationDate) Len() int {
	return len(s)
}
func (s byCreationDate) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}
func (s byCreationDate) Less(i, j int) bool {
	return s[i].PR.GetCreatedAt().After(s[j].PR.GetCreatedAt())
}

// fetchEEPR tries to get the mirroring PR in mesosphere/dcos-enterprise of a PR that is in dcos/dcos. Be aware that
// mergebot may not have picked up the OSS PR, yet. In this case no error is returned and the PR pointer is nil.
func fetchEEPR(client *github.Client, pr github.Issue, out chan<- github.Issue) {
	owner, repo, _ := getOwnerAndRepoAndID(pr.GetURL())
	out <- pr
	if owner != "dcos" || repo != "dcos" {
		return
	}
	comments, _, err := client.Issues.ListComments(context.Background(), owner, repo, pr.GetNumber(), nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error fetching PR comments: %s\n", err)
		return
	}
	for _, comment := range comments {

		eePRBody := regexp.MustCompile("^Enterprise Bump PR: (.*)$").FindStringSubmatch(comment.GetBody())
		if comment.GetUser().GetLogin() == "mesosphere-mergebot" && len(eePRBody) == 2 {
			owner, repo, sid := getOwnerAndRepoAndID(eePRBody[1])
			id, err := strconv.Atoi(sid)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error converting EE PR ID %s", sid)
				return
			}
			eepr, _, err := client.Issues.Get(context.Background(), owner, repo, id)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error getting EE PR %d", id)
				return
			}
			if eepr != nil {
				eepr.User = pr.GetUser()
				out <- *eepr
			}
		}
	}
}

func getTeamsPRs(client *github.Client, config config.Config) <-chan github.Issue {
	out := make(chan github.Issue)
	go func() {
		authors := make([]string, len(config.Teammates)+1)
		authors[0] = "author:" + config.User
		for idx, mate := range config.Teammates {
			authors[idx+1] = "author:" + mate
		}
		prs, _, err := client.Search.Issues(context.Background(), fmt.Sprintf("%s is:pr is:open org:dcos org:mesosphere", strings.Join(authors, " ")), nil)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error fetching PRs: %s\n", err)
			os.Exit(1)
		}

		for _, issue := range prs.Issues {
			out <- issue
		}
		close(out)
	}()
	return out
}

func prTooOld(pr github.Issue) bool {
	return pr.GetUpdatedAt().Add(24 * time.Hour * 90).Before(time.Now())
}

func findReviews(client *github.Client, user string, pr github.Issue, myPRs chan<- prs.PRWithReviews, otherPRs chan<- prs.PRWithReviews) {
	owner, repo, _ := getOwnerAndRepoAndID(pr.GetURL())
	reviews, _, err := client.PullRequests.ListReviews(context.Background(), owner, repo, pr.GetNumber(), nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error fetching reviews: %s\n", err)
		os.Exit(1)
	}
	sort.Sort(bySubmitDate(reviews))
	if pr.GetUser().GetLogin() == user {
		// this is a PR by me, get all the approving reviews on it.
		prWithReviews := prs.PRWithReviews{
			PR: pr,
		}
		for _, review := range reviews {
			if review.GetState() == "APPROVED" {
				prWithReviews.Reviews = append(prWithReviews.Reviews, review)
			}
		}
		myPRs <- prWithReviews
		return
	}
	// this is a PR from a team member. Check whether I have already reviewed.
	approved := false
	for _, review := range reviews {
		if review.GetUser().GetLogin() == user {
			if review.GetState() == "APPROVED" {
				approved = true
			}
		}
	}
	if !approved {
		otherPRs <- prs.PRWithReviews{
			PR: pr,
		}
	}
}

func processPRs(client *github.Client, user string, in <-chan github.Issue) <-chan struct{} {
	// this channel is closed as soon as processing is done
	fin := make(chan struct{})
	// start goroutine for processing and immediately exit
	go func() {
		var wg sync.WaitGroup
		myPRsCh := make(chan prs.PRWithReviews)
		otherPRsCh := make(chan prs.PRWithReviews)
		processFin := make(chan struct{})
		// start goroutine for collecting all found PRs and printing them to stdout
		go func() {
			myPRs := make([]prs.PRWithReviews, 0)
			otherPRs := make([]prs.PRWithReviews, 0)
			for {
				select {
				case pr, ok := <-myPRsCh:
					if !ok {
						// channel has been closed
						myPRsCh = nil
					} else {
						myPRs = append(myPRs, pr)
					}
				case pr, ok := <-otherPRsCh:
					if !ok {
						// channel has been closed
						otherPRsCh = nil
					} else {
						otherPRs = append(otherPRs, pr)
					}
				default:
					if myPRsCh == nil && otherPRsCh == nil {
						// all channels have been closed so we can start generating user output
						sort.Sort(byCreationDate(myPRs))
						sort.Sort(byCreationDate(otherPRs))
						table.PrintPRs(myPRs, otherPRs)
						close(processFin)
						return
					}
				}
			}
		}()
		for pr := range in {
			wg.Add(1)
			go func(pr github.Issue) {
				defer wg.Done()
				time.Sleep(3 * time.Second)
				findReviews(client, user, pr, myPRsCh, otherPRsCh)
			}(pr)
		}
		wg.Wait()
		close(myPRsCh)
		close(otherPRsCh)
		<-processFin
		close(fin)
	}()
	return fin
}

func main() {

	// initialize configuration

	config, err := config.Init()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading configuration: %s\n", err)
		os.Exit(1)
	}

	// initialize GitHub client library

	ctx := context.Background()
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: config.Token})
	tc := oauth2.NewClient(ctx, ts)

	client := github.NewClient(tc)

	// fetch PRs and commence work

	allPRs := make(chan github.Issue)
	// start function that further processes all incoming tasks on the allPRs channel
	fin := processPRs(client, config.User, allPRs)
	var wg sync.WaitGroup
	for teamPR := range getTeamsPRs(client, *config) {
		wg.Add(1)
		go func(pr github.Issue) {
			defer wg.Done()
			if prTooOld(pr) {
				return
			}
			fetchEEPR(client, pr, allPRs)
		}(teamPR)
	}

	// wait for all fetchEEPR goroutines to finish
	wg.Wait()
	// signal to processPRs that no further PR is to be expected
	close(allPRs)
	// wait for processPRs to finish work
	<-fin
}
