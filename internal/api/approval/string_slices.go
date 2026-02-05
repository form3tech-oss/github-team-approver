package approval

func appendIfMissing(s []string, v string) []string {
	for _, e := range s {
		if e == v {
			return s
		}
	}
	return append(s, v)
}

func uniqueAppend(a []string, b []string) []string {
	m := map[string]bool{}
	for _, e := range a {
		m[e] = true
	}

	for _, e := range b {
		if _, ok := m[e]; !ok {
			a = append(a, e)
		}
	}
	return a
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

func intersection(a, b []string) []string {
	m := map[string]bool{}
	for _, e := range a {
		m[e] = true
	}
	result := make([]string, 0, len(a))
	for _, e := range b {
		if _, ok := m[e]; ok {
			result = append(result, e)
			delete(m, e) // avoid duplicates
		}
	}
	return result
}
