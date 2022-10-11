package approval

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTruncate(t *testing.T) {
	tests := map[string]struct {
		v        string
		n        int
		expected string
	}{
		"expecting empty": {
			v:        "foo",
			n:        0,
			expected: "",
		},
		"expecting one char": {
			v:        "bar",
			n:        1,
			expected: "b",
		},
		"no truncation expected": {
			v:        "baz",
			n:        3,
			expected: "baz",
		},
		"expecting truncation with suffix": {
			v:        "foobar",
			n:        4,
			expected: "f...",
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, tt.expected, truncate(tt.v, tt.n))
		})
	}
}
