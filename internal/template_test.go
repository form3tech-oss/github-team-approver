package internal

import (
	"encoding/json"
	"github.com/google/go-github/v28/github"
	"strings"
	"testing"
)

func TestTemplate(t *testing.T){

	b := readGitHubExampleFile("pull_request_closed.json")

	var (
		e github.PullRequestEvent
	)

	if err := json.Unmarshal(b, &e); err != nil {
		t.Fatalf("could not unmarshall event, error: %v", err)
	}

	template := `
Action: {{.Action}}
User:  {{.PullRequest.User.Login}}
`

	var event event
	event = &e
	output, err := renderTemplate(event, template)
	if err != nil {
		t.Fatal(err)
	}

	expected := `
Action: closed
User:  kevholditch
`
	if !strings.EqualFold(string(output), expected) {
		t.Errorf("template not rendered correctly, expected: %s, got: %s", expected, output)
	}


}