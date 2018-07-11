package main

import (
	"bytes"
	"flag"
	"fmt"
	"github.com/google/go-github/github"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"text/template"
)

type roundTripper struct {
	accessToken string
}

func (rt roundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	r.Header.Set("Authorization", fmt.Sprintf("token %s", rt.accessToken))
	return http.DefaultTransport.RoundTrip(r)
}

var (
	token       = flag.String("token", os.Getenv("GITHUB_TOKEN"), "Github access token")
	owner       = flag.String("owner", os.Getenv("GITHUB_OWNER"), "Github repository owner")
	repo        = flag.String("repo", os.Getenv("GITHUB_REPO"), "Github repository name")
	commentType = flag.String("type", os.Getenv("GITHUB_COMMENT_TYPE"), "Comment type: 'commit', 'pr' or 'issue'")
	sha         = flag.String("sha", os.Getenv("GITHUB_COMMIT_SHA"), "Commit SHA")
	number      = flag.String("number", os.Getenv("GITHUB_PR_ISSUE_NUMBER"), "Pull Request or Issue number")
	format      = flag.String("format", os.Getenv("GITHUB_COMMENT_FORMAT"), "Comment format. Supports 'Go' templates: My comment:<br/>{{.}}")
	comment     = flag.String("comment", os.Getenv("GITHUB_COMMENT"), "Comment text")
)

func getPullRequestOrIssueNumber(str string) (int, error) {
	if str == "" {
		return 0, errors.New("-number or GITHUB_PR_ISSUE_NUMBER required")
	}

	num, err := strconv.Atoi(str)
	if err != nil {
		return 0, errors.WithMessage(err, "-number or GITHUB_PR_ISSUE_NUMBER must be an integer")
	}

	return num, nil
}

func getComment() (string, error) {
	// Read the comment from the command-line argument or ENV var first
	if *comment != "" {
		return *comment, nil
	}

	// If not provided in the command-line argument or ENV var, try to read from Stdin
	info, err := os.Stdin.Stat()
	if err != nil {
		return "", err
	}

	// Makes sure we have an input pipe, and it actually contains some bytes
	if info.Mode()&os.ModeCharDevice != 0 || info.Size() <= 0 {
		return "", errors.New("Comment must be provided either as command-line argument, ENV variable, or from 'Stdin'")
	}

	data, err := ioutil.ReadAll(os.Stdin)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

func formatComment(comment string) (string, error) {
	if *format == "" {
		return comment, nil
	}

	t, err := template.New("formatComment").Parse(*format)
	if err != nil {
		return "", err
	}

	var doc bytes.Buffer

	err = t.Execute(&doc, comment)
	if err != nil {
		return "", err
	}

	return doc.String(), nil
}

func main() {
	flag.Parse()

	if *token == "" {
		flag.PrintDefaults()
		log.Fatal("-token or GITHUB_TOKEN required")
	}
	if *owner == "" {
		flag.PrintDefaults()
		log.Fatal("-owner or GITHUB_OWNER required")
	}
	if *repo == "" {
		flag.PrintDefaults()
		log.Fatal("-repo or GITHUB_REPO required")
	}
	if *commentType == "" {
		flag.PrintDefaults()
		log.Fatal("-type or GITHUB_COMMENT_TYPE required")
	}
	if *commentType != "commit" && *commentType != "pr" && *commentType != "issue" {
		flag.PrintDefaults()
		log.Fatal("-type or GITHUB_COMMENT_TYPE must be one of 'commit', 'pr' or 'issue'")
	}

	http.DefaultClient.Transport = roundTripper{*token}
	githubClient := github.NewClient(http.DefaultClient)

	if *commentType == "commit" {
		if *sha == "" {
			flag.PrintDefaults()
			log.Fatal("-sha or GITHUB_COMMIT_SHA required")
		}

		comment, err := getComment()
		if err != nil {
			log.Fatal(err)
		}

		formattedComment, err := formatComment(comment)
		if err != nil {
			log.Fatal(err)
		}

		// https://developer.github.com/v3/repos/comments
		commitComment := &github.RepositoryComment{Body: &formattedComment}
		commitComment, _, err = githubClient.Repositories.CreateComment(context.Background(), *owner, *repo, *sha, commitComment)
		if err != nil {
			log.Fatal(err)
		}

		fmt.Println("github-commenter: created GitHub Commit comment", commitComment.ID)
	} else if *commentType == "pr" {
		num, err := getPullRequestOrIssueNumber(*number)
		if err != nil {
			log.Fatal(err)
		}

		comment, err := getComment()
		if err != nil {
			log.Fatal(err)
		}

		formattedComment, err := formatComment(comment)
		if err != nil {
			log.Fatal(err)
		}

		// https://developer.github.com/v3/pulls/comments
		// https://developer.github.com/v3/pulls/reviews/#create-a-pull-request-review
		pullRequestReviewComment := &github.PullRequestReviewRequest{Body: &formattedComment, Event: github.String("COMMENT")}
		pullRequestReview, _, err := githubClient.PullRequests.CreateReview(context.Background(), *owner, *repo, num, pullRequestReviewComment)
		if err != nil {
			log.Fatal(err)
		}

		fmt.Println("github-commenter: created GitHub PR Review comment", pullRequestReview.ID)
	} else if *commentType == "issue" {
		num, err := getPullRequestOrIssueNumber(*number)
		if err != nil {
			log.Fatal(err)
		}

		comment, err := getComment()
		if err != nil {
			log.Fatal(err)
		}

		formattedComment, err := formatComment(comment)
		if err != nil {
			log.Fatal(err)
		}

		// https://developer.github.com/v3/issues/comments
		issueComment := &github.IssueComment{Body: &formattedComment}
		issueComment, _, err = githubClient.Issues.CreateComment(context.Background(), *owner, *repo, num, issueComment)
		if err != nil {
			log.Fatal(err)
		}

		fmt.Println("github-commenter: created GitHub Issue comment", issueComment.ID)
	}
}