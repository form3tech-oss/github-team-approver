package github_test

import (
	"testing"

	"github.com/form3tech-oss/github-team-approver/internal/api/github/stages"
)

func TestDeletePRCommentWhenCommentExist(t *testing.T) {
	given, when, then := stages.ClientTest(t)

	given.
		FakeGHRunning().
		Organisation().
		Repo().
		PR().
		CommentExists()
	when.
		DeletingComment()
	then.
		ExpectCommentDeleted()
}

func TestDeletePRCommentWhenCommentAlreadyDeleted(t *testing.T) {
	given, when, then := stages.ClientTest(t)

	given.
		FakeGHRunning().
		Organisation().
		Repo().
		PR().
		CommentsDeleted()
	when.
		DeletingComment()
	then.
		ExpectCommentDeleted()
}
