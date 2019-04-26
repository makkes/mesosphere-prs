package table

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/go-github/v25/github"

	"github.com/makkes/prs/prs"
)

func pad(s string, width int) string {
	if width < 0 {
		return s
	}
	return fmt.Sprintf(fmt.Sprintf("%%-%ds", width), s)
}

func calcColumnWidths(prList []prs.PRWithReviews, now time.Time) map[string]int {
	res := make(map[string]int)
	res["owner"] = 20
	res["title"] = 50
	res["url"] = 10
	res["reviewers"] = 30
	for _, p := range prList {
		w := len(p.PR.GetUser().GetLogin())
		if w > res["owner"] {
			res["owner"] = w
		}

		w = len(p.PR.GetHTMLURL())
		if w > res["url"] {
			res["url"] = w
		}

		w = len(fmt.Sprintf(""))
		if w > res["reviewers"] {
			res["reviewers"] = w
		}
	}
	return res
}

func filterUsers(reviews []*github.PullRequestReview) []string {
	res := make([]string, 0)
	for _, review := range reviews {
		res = append(res, review.GetUser().GetLogin())
	}
	return res
}

func PrintPRs(myPRs []prs.PRWithReviews, otherPRs []prs.PRWithReviews) {
	widths := calcColumnWidths(append(myPRs, otherPRs...), time.Now())
	// for c, w := range widths {
	// 	fmt.Printf("%s: %d\n", c, w)
	// }
	fmt.Printf("%s %s %s %s\n",
		pad("OWNER", widths["owner"]),
		pad("TITLE", widths["title"]),
		pad("URL", widths["url"]),
		pad("REVIEWERS", widths["reviewers"]))
	printPRs(myPRs, widths)
	printPRs(otherPRs, widths)
}

func printPRs(prs []prs.PRWithReviews, widths map[string]int) {
	for _, p := range prs {
		title := p.PR.GetTitle()
		if len(title) > widths["title"] {
			title = title[0:widths["title"]-1] + "…"
		}
		fmt.Printf("%s %s %s %s\n",
			pad(p.PR.GetUser().GetLogin(), widths["owner"]),
			pad(title, widths["title"]),
			pad(p.PR.GetHTMLURL(), widths["url"]),
			pad(strings.Join(filterUsers(p.Reviews), ", "), widths["reviewers"]))
	}
}
