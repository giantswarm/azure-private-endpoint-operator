package privateendpoints

func sliceContains[T1, T2 any](items []T1, t T2, equal func(a T1, b T2) bool) bool {
	for _, item := range items {
		if equal(item, t) {
			return true
		}
	}

	return false
}
