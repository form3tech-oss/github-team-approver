package main

import (
	"encoding/json"
	"fmt"
	"github.com/form3tech-oss/github-team-approver/internal"
	"github.com/google/go-github/v28/github"
	"github.com/slack-go/slack"
	"io/ioutil"
	"os"
)

// use to test rendering a PR event with a template and sending to slack
// example
// ./hooktest http://slack/bar ./examples/templates/merged.template /examples/github/pull_request_closed.json
func main() {

	err := sendHook(os.Args); if err != nil {
		fmt.Printf("could not send hook, error: %v", err)
		os.Exit(-1)
	}
}

type event interface {
	GetAction() string
	GetPullRequest() *github.PullRequest
	GetRepo() *github.Repository
}

func sendHook(args [] string) error {
	if len(args) != 4 {
		return fmt.Errorf("you need to pass 4 arguments in format: ./hooktest http://slack/bar ./examples/templates/merged.template /examples/github/pull_request_closed.json")
	}
	slackHook := args[1]
	templatePath := args[2]
	githubExamplePath := args[3]

	t, err := ioutil.ReadFile(templatePath)
	if err != nil {
		return fmt.Errorf("could not load template: error: %v", err)
	}

	gh, err := ioutil.ReadFile(githubExamplePath)
	if err != nil {
		return fmt.Errorf("could not load github example: error: %v", err)
	}

	var e github.PullRequestEvent
	err = json.Unmarshal(gh, &e); if err != nil {
		return err
	}

	var event event
	event = &e

	rendered, err := internal.Render(event, string(t)); if err != nil {
		return fmt.Errorf("could not render template, error: %v", err)
	}

	var msg slack.WebhookMessage
	err = json.Unmarshal(rendered, &msg); if err != nil {
		return err
	}

	if err := slack.PostWebhook(slackHook, &msg); err != nil {
		return err
	}

	return nil
}