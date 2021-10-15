package fakegithub

import "fmt"

func (f *FakeGitHub) repoURL() string {
	return fmt.Sprintf("%s/%s", f.URL(), f.repoFullName())
}

func (f *FakeGitHub) contentsURL(dir string) string {
	return fmt.Sprintf("/repos/%s/contents/%s", f.repoFullName(), dir)
}

func (f *FakeGitHub) fileURL(branch, filename string) string {
	return fmt.Sprintf("/%s/%s/%s", f.repoFullName(), branch, filename)
}

func (f *FakeGitHub) teamsURL() string {
	return fmt.Sprintf("/orgs/%s/teams", f.org.OwnerName)
}

func (f *FakeGitHub) commitsURL() string {
	return fmt.Sprintf("/repos/%s/pulls/%d/commits", f.repoFullName(), f.pr.PRNumber)
}

func (f *FakeGitHub) reviewsURL() string {
	return fmt.Sprintf("/repos/%s/pulls/%d/reviews", f.repoFullName(), f.pr.PRNumber)
}

func (f *FakeGitHub) statusURL() string {
	return fmt.Sprintf("/repos/%s/statuses/%s", f.repoFullName(), f.pr.PRCommit)
}

func (f *FakeGitHub) labelsURL() string {
	return fmt.Sprintf("/repos/%s/issues/%d/labels", f.repoFullName(), f.pr.PRNumber)
}

func (f *FakeGitHub) commentsURL() string {
	return fmt.Sprintf("/repos/%s/issues/%d/comments", f.repoFullName(), f.pr.PRNumber)
}

func (f *FakeGitHub) issueCommentsURL() string {
	return fmt.Sprintf("/repos/%s/issues/comments/{id:[0-9]+}", f.repoFullName())
}

func (f *FakeGitHub) requestedReviewersURL() string {
	return fmt.Sprintf("/repos/%s/pulls/%d/requested_reviewers", f.repoFullName(), f.pr.PRNumber)
}

func (f *FakeGitHub) repoFullName() string {
	return fmt.Sprintf("%s/%s", f.org.OwnerName, f.repo.Name)
}