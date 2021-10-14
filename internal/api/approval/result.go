package approval

import "fmt"

const (
	statusEventDescriptionMaxLength = 140
)

type Result struct {
	status             string
	description        string
	finalLabels        []string
	reviewsToRequest   []string
	dismissedReviewers []string
}

func (r *Result) pendingReviewsWaiting() bool {
	return r.status == StatusEventStatusPending
}

func (r *Result) Description() string {
	return truncate(r.description, statusEventDescriptionMaxLength)
}

func (r *Result) Status() string               { return r.status }
func (r *Result) FinalLabels() []string        { return r.finalLabels }
func (r *Result) ReviewsToRequest() []string   { return r.reviewsToRequest }
func (r *Result) DismissedReviewers() []string { return r.dismissedReviewers }

func truncate(v string, n int) string {
	suffix := "..."

	if n <= len(suffix) {
		return v[:n]
	}

	if len(v) > n {
		return fmt.Sprintf("%s%s", v[:n-len(suffix)], suffix)
	}

	return v
}
