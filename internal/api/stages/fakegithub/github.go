package fakegithub

import (
	"fmt"
	"net/http/httptest"
	"testing"

	approverCfg "github.com/form3tech-oss/github-team-approver-commons/v2/pkg/configuration"
	"github.com/google/go-github/v42/github"

	"github.com/gorilla/mux"
)

type Team map[int64][]*github.User

type Org struct {
	// Teams present for the organisation
	OrgDetails  *github.Organization
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
	Files    []PRFile
}

type PRFile struct {
	SHA         string
	Filename    string
	ContentsURL string
}

type FakeGitHub struct {
	ts  *httptest.Server
	mux *mux.Router
	t   *testing.T

	org  *Org
	repo *Repo
	pr   *PR

	commits       []*github.RepositoryCommit
	reviews       []*github.PullRequestReview
	issueComments []*github.IssueComment
	events        []*github.IssueEvent

	reportedStatus         *github.RepoStatus
	reportedComments       []*github.IssueComment
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

	// only expose handlers when expected data is there
	f.mux.HandleFunc(f.teamsURL(), f.teamsHandler)
	f.mux.HandleFunc("/orgs/{org:.*}", f.orgsHandler)
	f.mux.HandleFunc("/organizations/{orgid:[0-9]+}/team/{id:[0-9]+}/members", f.teamsMemberHandler)
}

func (f *FakeGitHub) SetRepo(r *Repo) {
	f.repo = r
	f.mux.HandleFunc(f.contentsURL(approverCfg.ConfigurationFilePath), f.contentsHandler)
}

func (f *FakeGitHub) SetPR(pr *PR) {
	f.pr = pr

	// only expose handlers when expected data is there
	// the following handlers handles reporting (POST/PUT) from Approver Bot
	f.mux.HandleFunc(f.statusURL(), f.statusHandler)
	f.mux.HandleFunc(f.labelsURL(), f.labelsHandler)
	f.mux.HandleFunc(f.requestedReviewersURL(), f.requestedReviewersHandler)
	f.mux.HandleFunc(f.prFilesURL(), f.prFilesHandler)
}

func (f *FakeGitHub) SetCommits(r []*github.RepositoryCommit) {
	f.commits = r

	// only expose handlers when expected data is there
	f.mux.HandleFunc(f.commitsURL(), f.commitsHandler)
}

func (f *FakeGitHub) SetEvents(r []*github.IssueEvent) {
	f.events = r
	f.mux.HandleFunc(f.issueEventsURL(), f.issueEventsHandler)
}

func (f *FakeGitHub) SetReviews(r []*github.PullRequestReview) {
	f.reviews = append(f.reviews, r...)

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

func (f *FakeGitHub) ReportedLabels() []string                 { return f.reportedLabels }
func (f *FakeGitHub) ReportedStatus() *github.RepoStatus       { return f.reportedStatus }
func (f *FakeGitHub) ReportedComments() []*github.IssueComment { return f.reportedComments }
func (f *FakeGitHub) RequestedTeamReviews() []string           { return f.requestedTeamReviewers }
func (f *FakeGitHub) Comments() []*github.IssueComment         { return f.issueComments }

func (f *FakeGitHub) URL() string {
	return fmt.Sprintf("%s", f.ts.URL)
}
