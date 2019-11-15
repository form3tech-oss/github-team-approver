package function

import (
	"github.com/google/go-github/github"
)

func getLabelNames(labels []*github.Label) []string {
	if labels == nil {
		return make([]string, 0, 0)
	}
	r := make([]string, 0, len(labels))
	for _, l := range labels {
		r = append(r, l.GetName())
	}
	return r
}
