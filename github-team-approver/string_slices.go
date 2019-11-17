package function

func appendIfMissing(s []string, v string) []string {
	for _, e := range s {
		if e == v {
			return s
		}
	}
	return append(s, v)
}

func deleteIfExisting(s []string, v string) []string {
	i := indexOf(s, v)
	if i == -1 {
		return s
	}
	return append(s[:i], s[i+1:]...)
}

func indexOf(s []string, v string) int {
	for i, e := range s {
		if e == v {
			return i
		}
	}
	return -1
}
