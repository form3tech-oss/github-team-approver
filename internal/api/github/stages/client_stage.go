package stages

import (
	"context"
	ghclient "github.com/form3tech-oss/github-team-approver/internal/api/github"
	"github.com/form3tech-oss/github-team-approver/internal/api/secret"
	"github.com/form3tech-oss/github-team-approver/internal/api/stages/fakegithub"
	"github.com/google/go-github/v42/github"
	"github.com/stretchr/testify/require"
	"os"
	"testing"
)

type ClientStage struct {
	t          *testing.T
	fakeGitHub *fakegithub.FakeGitHub

	commentID  int64
	commentMsg string

	response    *github.Response
	errResponse error
}

func ClientTest(t *testing.T) (*ClientStage, *ClientStage, *ClientStage) {
	s := &ClientStage{
		t: t,
	}

	return s, s, s
}

func (c *ClientStage) FakeGHRunning() *ClientStage {
	c.fakeGitHub = fakegithub.NewFakeGithub(c.t)
	c.setupEnv("GITHUB_BASE_URL", c.fakeGitHub.URL())

	return c
}

func (c *ClientStage) Organisation() *ClientStage {
	c.fakeGitHub.SetOrg(&fakegithub.Org{
		OwnerName: "form3tech",
	})

	return c
}

func (c *ClientStage) Repo() *ClientStage {
	c.fakeGitHub.SetRepo(&fakegithub.Repo{
		Name: "some-repo",
	})

	return c
}

func (c *ClientStage) PR() *ClientStage {
	c.fakeGitHub.SetPR(&fakegithub.PR{
		PRNumber: 1,
	})

	return c
}

func (c *ClientStage) setupEnv(k, v string) {
	c.t.Cleanup(func() {
		err := os.Unsetenv(k)
		require.NoError(c.t, err)
	})
	err := os.Setenv(k, v)
	require.NoError(c.t, err)
}

func (c *ClientStage) DeletingComment() *ClientStage {
	secret := secret.NewEnvSecretStore()
	gc := ghclient.New(secret)
	resp, err := gc.DeletePRComment(
		context.TODO(),
		c.fakeGitHub.Org().OwnerName,
		c.fakeGitHub.Repo().Name,
		c.commentID)

	c.response = resp
	c.errResponse = err

	return c
}

func (c *ClientStage) ExpectCommentDeleted() *ClientStage {
	require.NotNil(c.t, c.response)
	require.NoError(c.t, c.errResponse)
	require.Empty(c.t, c.fakeGitHub.Comments())
	return c
}

func (c *ClientStage) CommentExists() {
	c.commentID = int64(42)
	c.commentMsg = "some message"
	c.fakeGitHub.SetIssueComments([]*github.IssueComment{
		{
			ID:   github.Int64(c.commentID),
			Body: github.String(c.commentMsg),
		},
	})
}

func (c *ClientStage) CommentsDeleted() {
	c.commentID = int64(42)
	c.commentMsg = "some message"
	c.fakeGitHub.SetIssueComments([]*github.IssueComment{})
}
