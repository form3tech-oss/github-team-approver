package approval

import (
	"testing"

	"github.com/google/go-github/v42/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestFindCoAuthors(t *testing.T) {
	tests := map[string]struct {
		message  string
		expected []string
	}{
		"when no co-authors in commit message": {
			message:  "feat: awesome new feature",
			expected: []string{},
		},
		"when one co-author in commit message": {
			message: `feat: awesome new feature

			Co-authored-by: John Doe <12345678+john-doe@users.noreply.github.com>`,
			expected: []string{"john-doe"},
		},
		"when one co-author in commit message (legacy format)": {
			message: `feat: awesome new feature

			Co-authored-by: John Doe <john-doe@users.noreply.github.com>`,
			expected: []string{"john-doe"},
		},
		"when multiple co-authors in commit message": {
			message: `feat: awesome new feature

			Co-authored-by: John Doe <12345678+john-doe@users.noreply.github.com>
			Co-authored-by: Jane Doe <87654321+jane-doe@users.noreply.github.com>`,
			expected: []string{"john-doe", "jane-doe"},
		},
		"when multiple co-authors in commit message (legacy format)": {
			message: `feat: awesome new feature

			Co-authored-by: John Doe <12345678+john-doe@users.noreply.github.com>
			Co-authored-by: Jane Doe <jane-doe@users.noreply.github.com>`,
			expected: []string{"john-doe", "jane-doe"},
		},
		"when one of the co-authors uses unsupported format": {
			message: `feat: awesome new feature

			Co-authored-by: John Doe <john@doe.com>
			Co-authored-by: Jane Doe <87654321+jane-doe@users.noreply.github.com>`,
			expected: []string{"jane-doe"},
		},
		"when duplicated co-author in commit message": {
			message: `feat: awesome new feature
			Co-authored-by: John Doe <12345678+john-doe@users.noreply.github.com>
			Co-authored-by: John Doe <12345678+john-doe@users.noreply.github.com>
			Co-authored-by: John Doe <12345678+john-doe@users.noreply.github.com>
			Co-authored-by: John Doe <12345678+john-doe@users.noreply.github.com>`,
			expected: []string{"john-doe"},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			require.ElementsMatch(t, tt.expected, findCoAuthors(tt.message))
		})
	}
}

func TestFindReopeners(t *testing.T) {
	tests := map[string]struct {
		events   []*github.IssueEvent
		expected map[string]bool
	}{
		"When there are no reopen events": {
			[]*github.IssueEvent{
				{
					Actor: &github.User{Login: github.String("foo")},
					Event: github.String("closed"),
				},
				{
					Actor: &github.User{Login: github.String("bar")},
					Event: github.String("merged"),
				},
			},
			map[string]bool{},
		},
		"When there are reopen events for one user": {
			[]*github.IssueEvent{
				{
					Actor: &github.User{Login: github.String("foo")},
					Event: github.String("reopened"),
				},
				{
					Actor: &github.User{Login: github.String("bar")},
					Event: github.String("merged"),
				},
			},
			map[string]bool{
				"foo": true,
			},
		},
		"When there are reopen events for multiple users": {
			[]*github.IssueEvent{
				{
					Actor: &github.User{Login: github.String("foo")},
					Event: github.String("reopened"),
				},
				{
					Actor: &github.User{Login: github.String("bar")},
					Event: github.String("reopened"),
				},
			},
			map[string]bool{
				"foo": true,
				"bar": true,
			},
		},
		"When there are multiple events for a user including a reopen": {
			[]*github.IssueEvent{
				{
					Actor: &github.User{Login: github.String("foo")},
					Event: github.String("merged"),
				},
				{
					Actor: &github.User{Login: github.String("foo")},
					Event: github.String("reopened"),
				},
			},
			map[string]bool{
				"foo": true,
			},
		},
		"When there are multiple events for multiple users including a reopen": {
			[]*github.IssueEvent{
				{
					Actor: &github.User{Login: github.String("foo")},
					Event: github.String("merged"),
				},
				{
					Actor: &github.User{Login: github.String("foo")},
					Event: github.String("reopened"),
				},
				{
					Actor: &github.User{Login: github.String("bar")},
					Event: github.String("reopened"),
				},
				{
					Actor: &github.User{Login: github.String("foo")},
					Event: github.String("closed"),
				},
			},
			map[string]bool{
				"foo": true,
				"bar": true,
			},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, tt.expected, findReOpeners(tt.events))
		})
	}
}

func TestFilterAllowedAndIgnoreReviewers(t *testing.T) {
	tests := map[string]struct {
		commits []*github.RepositoryCommit
		events  []*github.IssueEvent
		members []*github.User
		allowed []string
		ignored []string
	}{
		"When no member is an author in PR": {
			[]*github.RepositoryCommit{
				{
					Committer: &github.User{Login: github.String("foo")},
				},
			},
			nil,
			[]*github.User{
				{Login: github.String("bar")},
			},
			[]string{"bar"},
			nil,
		},
		"When only member is an author in PR": {
			[]*github.RepositoryCommit{
				{
					Committer: &github.User{Login: github.String("foo")},
				},
			},
			nil,
			[]*github.User{
				{Login: github.String("foo")},
			},
			nil,
			[]string{"foo"},
		},
		"When multiple members exist without being author": {
			[]*github.RepositoryCommit{},
			nil,
			[]*github.User{
				{Login: github.String("foo")},
				{Login: github.String("bar")},
				{Login: github.String("baz")},
			},
			[]string{"bar", "baz", "foo"},
			nil,
		},
		"When multiple members exist, some are authors": {
			[]*github.RepositoryCommit{
				{
					Committer: &github.User{Login: github.String("bar")},
				},
				{
					Committer: &github.User{Login: github.String("qux")},
				},
			},
			nil,
			[]*github.User{
				{Login: github.String("foo")},
				{Login: github.String("bar")},
				{Login: github.String("baz")},
				{Login: github.String("qux")},
			},
			[]string{"baz", "foo"},
			[]string{"bar", "qux"},
		},
		"When multiple members exist, some are co-authors": {
			[]*github.RepositoryCommit{
				{
					Committer: &github.User{Login: github.String("bar")},
					Commit: &github.Commit{
						Message: github.String("feat: awesome new feature\n\nCo-authored-by: foo <12345678+foo@users.noreply.github.com>"),
					},
				},
				{
					Committer: &github.User{Login: github.String("qux")},
				},
			},
			nil,
			[]*github.User{
				{Login: github.String("foo")},
				{Login: github.String("bar")},
				{Login: github.String("baz")},
				{Login: github.String("qux")},
			},
			[]string{"baz"},
			[]string{"bar", "foo", "qux"},
		},
		"When no member is an author in PR and not a reopener": {
			[]*github.RepositoryCommit{
				{
					Committer: &github.User{Login: github.String("foo")},
				},
			},
			[]*github.IssueEvent{
				{
					Actor: &github.User{Login: github.String("baz")},
					Event: github.String("reopened"),
				},
			},
			[]*github.User{
				{Login: github.String("bar")},
			},
			[]string{"bar"},
			nil,
		},
		"When only member is a reopener in PR": {
			[]*github.RepositoryCommit{
				{
					Committer: &github.User{Login: github.String("foo")},
				},
			},
			[]*github.IssueEvent{
				{
					Actor: &github.User{Login: github.String("bar")},
					Event: github.String("reopened"),
				},
			},
			[]*github.User{
				{Login: github.String("bar")},
			},
			nil,
			[]string{"bar"},
		},
		"When multiple members exist without being author or reopener": {
			[]*github.RepositoryCommit{},
			[]*github.IssueEvent{
				{
					Actor: &github.User{Login: github.String("qux")},
					Event: github.String("reopened"),
				},
				{
					Actor: &github.User{Login: github.String("corge")},
					Event: github.String("reopened"),
				},
				{
					Actor: &github.User{Login: github.String("grault")},
					Event: github.String("reopened"),
				},
			},
			[]*github.User{
				{Login: github.String("foo")},
				{Login: github.String("bar")},
				{Login: github.String("baz")},
			},
			[]string{"bar", "baz", "foo"},
			nil,
		},
		"When multiple members exist, some are reopeners": {
			[]*github.RepositoryCommit{
				{
					Committer: &github.User{Login: github.String("corge")},
				},
				{
					Committer: &github.User{Login: github.String("grault")},
				},
			},
			[]*github.IssueEvent{
				{
					Actor: &github.User{Login: github.String("bar")},
					Event: github.String("reopened"),
				},
				{
					Actor: &github.User{Login: github.String("qux")},
					Event: github.String("reopened"),
				},
			},
			[]*github.User{
				{Login: github.String("foo")},
				{Login: github.String("bar")},
				{Login: github.String("baz")},
				{Login: github.String("qux")},
			},
			[]string{"baz", "foo"},
			[]string{"bar", "qux"},
		},
		"When multiple members exist, some are reopeners, and some are authors": {
			[]*github.RepositoryCommit{
				{
					Committer: &github.User{Login: github.String("corge")},
				},
				{
					Committer: &github.User{Login: github.String("grault")},
				},
			},
			[]*github.IssueEvent{
				{
					Actor: &github.User{Login: github.String("bar")},
					Event: github.String("reopened"),
				},
				{
					Actor: &github.User{Login: github.String("qux")},
					Event: github.String("reopened"),
				},
			},
			[]*github.User{
				{Login: github.String("foo")},
				{Login: github.String("bar")},
				{Login: github.String("baz")},
				{Login: github.String("qux")},
				{Login: github.String("corge")},
				{Login: github.String("grault")},
			},
			[]string{"baz", "foo"},
			[]string{"bar", "qux", "corge", "grault"},
		},
		"When multiple members exist, some are reopeners, some are co-authors, and one is the author": {
			[]*github.RepositoryCommit{
				{
					Committer: &github.User{Login: github.String("bar")},
					Commit: &github.Commit{
						Message: github.String("feat: awesome new feature\n\nCo-authored-by: foo <12345678+foo@users.noreply.github.com>"),
					},
				},
				{
					Committer: &github.User{Login: github.String("qux")},
				},
			},
			[]*github.IssueEvent{
				{
					Actor: &github.User{Login: github.String("grault")},
					Event: github.String("reopened"),
				},
				{
					Actor: &github.User{Login: github.String("corge")},
					Event: github.String("reopened"),
				},
			},
			[]*github.User{
				{Login: github.String("foo")},
				{Login: github.String("bar")},
				{Login: github.String("baz")},
				{Login: github.String("qux")},
				{Login: github.String("corge")},
				{Login: github.String("grault")},
			},
			[]string{"baz"},
			[]string{"bar", "foo", "qux", "corge", "grault"},
		},
		"When multiple members exist, co-author is reopener": {
			[]*github.RepositoryCommit{
				{
					Committer: &github.User{Login: github.String("bar")},
					Commit: &github.Commit{
						Message: github.String("feat: awesome new feature\n\nCo-authored-by: foo <12345678+foo@users.noreply.github.com>"),
					},
				},
				{
					Committer: &github.User{Login: github.String("qux")},
				},
			},
			[]*github.IssueEvent{
				{
					Actor: &github.User{Login: github.String("foo")},
					Event: github.String("reopened"),
				},
				{
					Actor: &github.User{Login: github.String("corge")},
					Event: github.String("reopened"),
				},
			},
			[]*github.User{
				{Login: github.String("foo")},
				{Login: github.String("bar")},
				{Login: github.String("baz")},
				{Login: github.String("qux")},
				{Login: github.String("corge")},
				{Login: github.String("grault")},
			},
			[]string{"baz", "grault"},
			[]string{"bar", "foo", "qux", "corge"},
		},
		"When multiple members exist, author is reopener": {
			[]*github.RepositoryCommit{
				{
					Committer: &github.User{Login: github.String("bar")},
					Commit: &github.Commit{
						Message: github.String("feat: awesome new feature\n\nCo-authored-by: foo <12345678+foo@users.noreply.github.com>"),
					},
				},
				{
					Committer: &github.User{Login: github.String("qux")},
				},
			},
			[]*github.IssueEvent{
				{
					Actor: &github.User{Login: github.String("bar")},
					Event: github.String("reopened"),
				},
				{
					Actor: &github.User{Login: github.String("corge")},
					Event: github.String("reopened"),
				},
			},
			[]*github.User{
				{Login: github.String("foo")},
				{Login: github.String("bar")},
				{Login: github.String("baz")},
				{Login: github.String("qux")},
				{Login: github.String("corge")},
				{Login: github.String("grault")},
			},
			[]string{"baz", "grault"},
			[]string{"bar", "foo", "qux", "corge"},
		},
	}

	for name, tt := range tests {
		t.Run(name,
			func(t *testing.T) {
				gotAllowed, gotIgnored := filterAllowedAndIgnoreReviewers(tt.members, tt.commits, tt.events)
				require.ElementsMatch(t, gotAllowed, tt.allowed)
				require.ElementsMatch(t, gotIgnored, tt.ignored)
			})
	}
}

// --- helper functions ---

func getCommitFiles(contentUrls ...string) []*github.CommitFile {

	var out []*github.CommitFile
	for i := range contentUrls {
		out = append(out, &github.CommitFile{ContentsURL: &contentUrls[i]})
	}
	return out
}
