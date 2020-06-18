package internal

import (
	"encoding/json"
	"fmt"
	"github.com/google/go-github/v28/github"
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
Image: 
`
	output, err := render(e, template)
	if err != nil {
		t.Fatal(err)
	}

	fmt.Print(output)


}