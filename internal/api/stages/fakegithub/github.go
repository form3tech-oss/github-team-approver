package fakegithub

import (
	"fmt"
	approverCfg "github.com/form3tech-oss/github-team-approver-commons/pkg/configuration"
	"github.com/google/go-github/v42/github"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
)

type Team map[int64][]*github.User

type Org struct {
	// Teams present for the organisation
	Teams       []*github.Team
	TeamMembers Team
	OwnerName   string
}

type Repo struct {
	Name        string
	ApproverCfg *approverCfg.Configuration
}

type PR struct {
	PRNumber int
	PRCommit string
}

type FakeGitHub struct {
	ts  *httptest.Server
	mux *mux.Router
	t   *testing.T

	ghorg *github.Organization

	org  *Org
	repo *Repo
	pr   *PR

	repoContents  []*github.RepositoryContent
	commits       []*github.RepositoryCommit
	reviews       []*github.PullRequestReview
	issueComments []*github.IssueComment

	reportedStatus         *github.RepoStatus
	reportedComment        *github.IssueComment
	reportedLabels         []string
	requestedTeamReviewers []string
}

func NewFakeGithub(t *testing.T) *FakeGitHub {
	m := mux.NewRouter()

	f := &FakeGitHub{
		ts:  httptest.NewServer(m),
		mux: m,
		t:   t,
	}
	t.Cleanup(f.Close)

	return f
}

func (f *FakeGitHub) Close() {
	f.ts.Close()
}

func (f *FakeGitHub) SetOrg(o *Org) {
	f.org = o
	id := int64(1)
	f.ghorg = &github.Organization{
		ID: &id,
	}

	// only expose handlers when expected data is there
	f.mux.HandleFunc(f.teamsURL(), f.teamsHandler)
	f.mux.HandleFunc("/orgs/{org:.*}", f.orgsHandler)
	f.mux.HandleFunc("/organizations/{orgid:[0-9]+}/team/{id:[0-9]+}/members", f.teamsMemberHandler)

}

func (f *FakeGitHub) SetRepo(r *Repo) {
	f.repo = r

	// only expose handlers when expected data is there
	f.mux.HandleFunc(f.fileURL("master", approverCfg.ConfigurationFilePath), f.configFileHandler)
}

func (f *FakeGitHub) SetPR(pr *PR) {
	f.pr = pr

	// only expose handlers when expected data is there
	// the following handlers handles reporting (POST/PUT) from Approver Bot
	f.mux.HandleFunc(f.statusURL(), f.statusHandler)
	f.mux.HandleFunc(f.labelsURL(), f.labelsHandler)
	f.mux.HandleFunc(f.requestedReviewersURL(), f.requestedReviewersHandler)

}

func (f *FakeGitHub) SetRepoContents(c []*github.RepositoryContent) {
	f.repoContents = c

	// only expose handlers when expected data is there
	f.mux.HandleFunc(f.contentsURL(".github"), f.contentsHandler)
}

func (f *FakeGitHub) SetCommits(r []*github.RepositoryCommit) {
	f.commits = r

	// only expose handlers when expected data is there
	f.mux.HandleFunc(f.commitsURL(), f.commitsHandler)
}

func (f *FakeGitHub) SetReviews(r []*github.PullRequestReview) {
	f.reviews = r

	// only expose handlers when expected data is there
	f.mux.HandleFunc(f.reviewsURL(), f.reviewsHandler)
}

func (f *FakeGitHub) SetIssueComments(c []*github.IssueComment) {
	f.issueComments = c

	// only expose handlers when expected data is there
	f.mux.HandleFunc(f.commentsURL(), f.commentsHandler)
	f.mux.HandleFunc(f.issueCommentsURL(), f.deleteCommentHandler)
}

func (f *FakeGitHub) Org() *Org   { return f.org }
func (f *FakeGitHub) Repo() *Repo { return f.repo }
func (f *FakeGitHub) PR() *PR     { return f.pr }

func (f *FakeGitHub) ReportedLabels() []string              { return f.reportedLabels }
func (f *FakeGitHub) ReportedStatus() *github.RepoStatus    { return f.reportedStatus }
func (f *FakeGitHub) ReportedComment() *github.IssueComment { return f.reportedComment }
func (f *FakeGitHub) RequestedTeamReviews() []string        { return f.requestedTeamReviewers }
func (f *FakeGitHub) Comments() []*github.IssueComment      { return f.issueComments }

func (f *FakeGitHub) URL() string {
	return fmt.Sprintf("%s", f.ts.URL)
}
