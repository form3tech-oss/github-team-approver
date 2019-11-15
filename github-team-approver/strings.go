package function

const (
	truncatedStringSuffix = "..."
)

func truncate(v string, n int) string {
	if n <= len(truncatedStringSuffix) {
		return v[:n]
	}
	r := v
	if len(v) > n {
		r = v[:n-len(truncatedStringSuffix)] + truncatedStringSuffix
	}
	return r
}
