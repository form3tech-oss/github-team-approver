package fakegithub

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"

	"github.com/google/go-github/v42/github"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/require"
)

func (f *FakeGitHub) contentsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if f.repo.ApproverCfg == nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	var buf bytes.Buffer
	err := f.repo.ApproverCfg.Write(&buf)
	require.NoError(f.t, err)

	content := &github.RepositoryContent{
		Content: github.String(buf.String()),
	}

	payload, err := json.Marshal(content)
	require.NoError(f.t, err)

	w.Header().Set("Content-Type", "application/json")
	_, err = w.Write(payload)
	require.NoError(f.t, err)
}

func (f *FakeGitHub) teamsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	payload, err := json.Marshal(f.org.Teams)
	require.NoError(f.t, err)
	_, err = w.Write(payload)
	require.NoError(f.t, err)
}

func (f *FakeGitHub) orgsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	payload, err := json.Marshal(f.org.OrgDetails)
	require.NoError(f.t, err)
	_, err = w.Write(payload)
	require.NoError(f.t, err)
}

func (f *FakeGitHub) teamsMemberHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	vars := mux.Vars(r)
	val, ok := vars["id"]
	require.True(f.t, ok, "go-github sent incorrect team ID. URL: %s", r.URL.String())
	id, err := strconv.Atoi(val)
	require.NoError(f.t, err)

	members, ok := f.org.TeamMembers[int64(id)]
	require.True(f.t, ok, "team ID has not been setup in fake github: %d", id)
	w.Header().Set("Content-Type", "application/json")
	payload, err := json.Marshal(members)
	require.NoError(f.t, err)
	_, err = w.Write(payload)
	require.NoError(f.t, err)
}

func (f *FakeGitHub) commitsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	require.NotEmpty(f.t, f.commits)

	w.Header().Set("Content-Type", "application/json")
	payload, err := json.Marshal(f.commits)
	require.NoError(f.t, err)
	_, err = w.Write(payload)
	require.NoError(f.t, err)
}

func (f *FakeGitHub) reviewsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	payload, err := json.Marshal(f.reviews)
	require.NoError(f.t, err)
	_, err = w.Write(payload)
	require.NoError(f.t, err)
}

func (f *FakeGitHub) statusHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	status := &github.RepoStatus{}
	payload, err := ioutil.ReadAll(r.Body)
	require.NoError(f.t, err)

	err = json.Unmarshal(payload, status)
	require.NoError(f.t, err)

	f.reportedStatus = status
	w.WriteHeader(http.StatusCreated)
}

func (f *FakeGitHub) labelsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	var labels []string
	payload, err := ioutil.ReadAll(r.Body)
	require.NoError(f.t, err)

	err = json.Unmarshal(payload, &labels)
	require.NoError(f.t, err)

	f.reportedLabels = labels
	w.Header().Set("Content-Type", "application/json")

	var ghLabels []*github.Label
	for _, l := range labels {
		ghLabels = append(ghLabels, &github.Label{Name: github.String(l)})
	}
	payload, err = json.Marshal(ghLabels)
	require.NoError(f.t, err)

	// ack by writing back to client
	_, err = w.Write(payload)
	require.NoError(f.t, err)

}

func (f *FakeGitHub) requestedReviewersHandler(w http.ResponseWriter, r *http.Request) {
	var reviewReq *github.ReviewersRequest
	payload, err := ioutil.ReadAll(r.Body)
	require.NoError(f.t, err)

	err = json.Unmarshal(payload, &reviewReq)
	require.NoError(f.t, err)

	f.requestedTeamReviewers = reviewReq.TeamReviewers

	w.Header().Set("Content-Type", "application/json")

	// ack by writing back an empty pr back to client
	// if we for any reason start using the response we need to populate it appropriately
	pr := &github.PullRequest{}
	payload, err = json.Marshal(pr)
	require.NoError(f.t, err)

	_, err = w.Write(payload)
	require.NoError(f.t, err)
}

func (f *FakeGitHub) prFilesHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	require.NotNil(f.t, f.pr)

	files := []*github.CommitFile{}
	for _, file := range f.pr.Files {
		repoPath := fmt.Sprintf("https://api.github.com/repos/%s/contents/%s", f.repoFullName(), file.Filename)
		files = append(files, &github.CommitFile{
			SHA:         github.String(file.SHA),
			Filename:    github.String(file.Filename),
			ContentsURL: github.String(repoPath),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	payload, err := json.Marshal(files)
	require.NoError(f.t, err)
	_, err = w.Write(payload)
	require.NoError(f.t, err)
}

func (f *FakeGitHub) issueEventsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	require.NotNil(f.t, f.events)

	w.Header().Set("Content-Type", "application/json")
	payload, err := json.Marshal(f.events)
	require.NoError(f.t, err)
	_, err = w.Write(payload)
	require.NoError(f.t, err)
}
