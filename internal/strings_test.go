package internal

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_truncate(t *testing.T) {
	tests := []struct {
		v        string
		n        int
		expected string
	}{
		{
			v:        "foo",
			n:        0,
			expected: "",
		},
		{
			v:        "bar",
			n:        1,
			expected: "b",
		},
		{
			v:        "baz",
			n:        3,
			expected: "baz",
		},
		{
			v:        "foobar",
			n:        4,
			expected: "f...",
		},
	}
	for _, test := range tests {
		assert.Equal(t, test.expected, truncate(test.v, test.n))
	}
}
