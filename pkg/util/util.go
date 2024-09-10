package util

func ConvertToStringSlice(pointers []*string) []string {
	result := make([]string, len(pointers))
	for i, p := range pointers {
		if p != nil {
			result[i] = *p
		}
	}
	return result
}
