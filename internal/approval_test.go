package internal

import (
	"github.com/google/go-github/v28/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestIsDirectoryChanged(t *testing.T) {

	t.Run("absolute directory and matching commit files returns true", func(t *testing.T) {

		commitFiles := getCommitFiles(
			"https://api.github.com/repos/octocat/Hello-World/production/file1.txt?ref=6dcb09b5b57875f334f61aebed695e2e4193db5e",
			"https://api.github.com/repos/octocat/Hello-World/staging/file1.txt?ref=6dcb09b5b57875f334f61aebed695e2e4193db5e",
		)
		changed, err := isDirectoryChanged("/production", commitFiles)

		require.NoError(t, err)
		assert.True(t, changed)
	})

	t.Run("absolute directory ending with '/' and matching commit files returns true", func(t *testing.T) {

		commitFiles := getCommitFiles(
			"https://api.github.com/repos/octocat/Hello-World/production/file1.txt?ref=6dcb09b5b57875f334f61aebed695e2e4193db5e",
			"https://api.github.com/repos/octocat/Hello-World/staging/file1.txt?ref=6dcb09b5b57875f334f61aebed695e2e4193db5e",
		)
		changed, err := isDirectoryChanged("/production/", commitFiles)

		require.NoError(t, err)
		assert.True(t, changed)
	})

	t.Run("absolute directory and not matching commits files returns false", func(t *testing.T) {

		// matching absolute '/production' but commit is to '/docs/production'
		commitFiles := getCommitFiles(
			"https://api.github.com/repos/octocat/Hello-World/docs/production/file1.txt?ref=6dcb09b5b57875f334f61aebed695e2e4193db5e",
			"https://api.github.com/repos/octocat/Hello-World/staging/file1.txt?ref=6dcb09b5b57875f334f61aebed695e2e4193db5e",
		)
		changed, err := isDirectoryChanged("/production", commitFiles)

		require.NoError(t, err)
		assert.False(t, changed)
	})

	t.Run("relative directory and matching commit files returns true", func(t *testing.T) {

		// matching relative 'production' and commit is to '/docs/production'
		commitFiles := getCommitFiles(
			"https://api.github.com/repos/octocat/Hello-World/docs/production/file1.txt?ref=6dcb09b5b57875f334f61aebed695e2e4193db5e",
		)
		changed, err := isDirectoryChanged("production", commitFiles)

		require.NoError(t, err)
		assert.True(t, changed)
	})

	t.Run("relative directory matching commit file name returns false", func(t *testing.T) {

		// matching relative 'production' and commit is to '/docs/production' file not directory
		commitFiles := getCommitFiles(
			"https://api.github.com/repos/octocat/Hello-World/docs/production?ref=6dcb09b5b57875f334f61aebed695e2e4193db5e",
		)
		changed, err := isDirectoryChanged("production", commitFiles)

		require.NoError(t, err)
		assert.False(t, changed)
	})

	t.Run("invalid commit files returns error", func(t *testing.T) {

		// matching relative 'production' and commit is to '/docs/production'
		commitFiles := getCommitFiles(
			"https://api.github.com/production/file1.txt?ref=6dcb09b5b57875f334f61aebed695e2e4193db5e",
		)
		_, err := isDirectoryChanged("production", commitFiles)

		require.Error(t, err)
	})

	t.Run("nil commit content url returns error", func(t *testing.T) {

		commitFiles := []*github.CommitFile{{}}
		_, err := isDirectoryChanged("production", commitFiles)

		require.Error(t, err)
	})

	t.Run("nil commit files returns false", func(t *testing.T) {

		changed, err := isDirectoryChanged("production", nil)

		require.NoError(t, err)
		assert.False(t, changed)
	})
}

func TestContentsUrlToRelDir(t *testing.T) {

	t.Run("valid contents url returns rel path", func(t *testing.T) {

		contentsUrl := "https://api.github.com/repos/octocat/Hello-World/contents/file1.txt?ref=6dcb09b5b57875f334f61aebed695e2e4193db5e"
		relPath, err := contentsUrlToRelDir(contentsUrl)

		require.NoError(t, err)
		assert.Equal(t, "contents", relPath)
	})

	t.Run("contents url with missing scheme (invalid url) returns error", func(t *testing.T) {

		contentsUrl := "://api.github.com/repos/octocat/Hello-World/contents/file1.txt?ref=6dcb09b5b57875f334f61aebed695e2e4193db5e"
		_, err := contentsUrlToRelDir(contentsUrl)

		require.Error(t, err)
	})

	t.Run("contents url without <org> and <repo> parts returns error", func(t *testing.T) {

		contentsUrl := "https://api.github.com/repos/file1.txt?ref=6dcb09b5b57875f334f61aebed695e2e4193db5e"
		_, err := contentsUrlToRelDir(contentsUrl)

		require.Error(t, err)
	})

	t.Run("contents url with missing 'repos' parent directory returns error", func(t *testing.T) {

		contentsUrl := "https://api.github.com/octocat/Hello-World/contents/file1.txt?ref=6dcb09b5b57875f334f61aebed695e2e4193db5e"
		_, err := contentsUrlToRelDir(contentsUrl)

		require.Error(t, err)
	})
}

// --- helper functions ---

func getCommitFiles(contentUrls ...string) []*github.CommitFile {

	var out []*github.CommitFile
	for i := range contentUrls {
		out = append(out, &github.CommitFile{ContentsURL: &contentUrls[i]})
	}
	return out
}
