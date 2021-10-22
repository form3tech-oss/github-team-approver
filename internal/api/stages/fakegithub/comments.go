package fakegithub

import (
	"encoding/json"
	"fmt"
	"github.com/google/go-github/v28/github"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/require"
	"io/ioutil"
	"net/http"
	"strconv"
)

func (f *FakeGitHub) commentsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		f.postCommentHandler(w, r)
		return
	}

	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	payload, err := json.Marshal(f.comments) // comments can be nil on purpose
	require.NoError(f.t, err)

	_, err = w.Write(payload)
	require.NoError(f.t, err)
}

func (f *FakeGitHub) postCommentHandler(w http.ResponseWriter, r *http.Request) {
	var comment *github.IssueComment
	payload, err := ioutil.ReadAll(r.Body)
	require.NoError(f.t, err)

	err = json.Unmarshal(payload, &comment)
	require.NoError(f.t, err)

	f.reportedComment = comment

	w.Header().Set("Content-Type", "application/json")

	// ack by writing payload back to client
	_, err = w.Write(payload)
	require.NoError(f.t, err)
}

func (f *FakeGitHub) deleteCommentHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		w.WriteHeader(http.StatusBadRequest)
		return
	}


	vars := mux.Vars(r)
	val, ok := vars["id"]
	require.True(f.t, ok, "go-github sent incorrect comment ID. URL: %s", r.URL.String())
	id, err := strconv.Atoi(val)
	require.NoError(f.t, err)

	err = f.deleteComment(int64(id))
	require.NoError(f.t, err)

	w.WriteHeader(http.StatusNoContent)
}

func (f *FakeGitHub) deleteComment(id int64) error {
	for i, c := range f.comments {
		if *c.ID == id {
			f.comments = append(f.comments[:i], f.comments[i+1:]...)

			return nil
		}
	}
	return fmt.Errorf("comment not found: %d", id)
}