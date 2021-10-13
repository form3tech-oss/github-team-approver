package api

import (
	"github.com/stretchr/testify/require"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_appendIfMissing(t *testing.T) {
	tests := []struct {
		s    []string
		v    string
		want []string
	}{
		{
			s:    nil,
			v:    "foo",
			want: []string{"foo"},
		},
		{
			s:    []string{},
			v:    "foo",
			want: []string{"foo"},
		},
		{
			s:    []string{"foo", "bar", "baz"},
			v:    "qux",
			want: []string{"foo", "bar", "baz", "qux"},
		},
		{
			s:    []string{"foo", "bar", "baz"},
			v:    "bar",
			want: []string{"foo", "bar", "baz"},
		},
	}
	for _, test := range tests {
		assert.Equal(t, test.want, appendIfMissing(test.s, test.v))
	}
}

func Test_deleteIfExisting(t *testing.T) {
	tests := []struct {
		s    []string
		v    string
		want []string
	}{
		{
			s:    nil,
			v:    "foo",
			want: nil,
		},
		{
			s:    []string{},
			v:    "foo",
			want: []string{},
		},
		{
			s:    []string{"foo", "bar", "baz"},
			v:    "bar",
			want: []string{"foo", "baz"},
		},
		{
			s:    []string{"foo", "bar", "baz"},
			v:    "qux",
			want: []string{"foo", "bar", "baz"},
		},
	}
	for _, test := range tests {
		assert.Equal(t, test.want, deleteIfExisting(test.s, test.v))
	}
}

func Test_indexOf(t *testing.T) {
	tests := []struct {
		s    []string
		v    string
		want int
	}{
		{
			s:    nil,
			v:    "foo",
			want: -1,
		},
		{
			s:    []string{},
			v:    "foo",
			want: -1,
		},
		{
			s:    []string{"foo", "bar", "baz"},
			v:    "bar",
			want: 1,
		},
		{
			s:    []string{"foo", "bar", "baz"},
			v:    "qux",
			want: -1,
		},
	}
	for _, test := range tests {
		assert.Equal(t, test.want, indexOf(test.s, test.v))
	}
}

func Test_uniqueAppend(t *testing.T) {
	tests := map[string]struct {
		a    []string
		b    []string
		want []string
	}{
		"add unique to nil": {
			nil,
			[]string{"foo"},
			[]string{"foo"},
		},
		"add nil to empty slice" : {
			[]string{},
			nil,
			[]string{},
		},
		"add unique to empty slice": {
			[]string{},
			[]string{"foo"},
			[]string{"foo"},
		},
		"add unique": {
			[]string{"foo", "bar", "baz"},
			[]string{"qux"},
			[]string{"foo", "bar", "baz", "qux"},
		},
		"add duplicate": {
			[]string{"foo", "bar", "baz"},
			[]string{"bar"},
			[]string{"foo", "bar", "baz"},
		},
		"add empty": {
			[]string{"foo", "bar", "baz"},
			[]string{},
			[]string{"foo", "bar", "baz"},
		},
		"add many duplicates and a unique": {
			[]string{"foo", "bar", "baz"},
			[]string{"foo", "foo", "baz", "qux"},
			[]string{"foo", "bar", "baz", "qux"},
		},
	}
	for name, tt := range tests {
		t.Run(name,
			func(t *testing.T) {
				require.Equal(t, uniqueAppend(tt.a, tt.b), tt.want)
			})
	}
}
