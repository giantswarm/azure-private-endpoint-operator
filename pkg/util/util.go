package util

func ContainsPtr(slice []*string, str string) bool {
	for _, v := range slice {
		if v != nil && *v == str {
			return true
		}
	}
	return false
}

func ConvertToStringSlice(pointers []*string) []string {
	result := make([]string, len(pointers))
	for i, p := range pointers {
		if p != nil {
			result[i] = *p
		}
	}
	return result
}
