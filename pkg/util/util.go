package util

import (
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/cluster-api/api/v1beta1"
)

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

// AreStatusConditionsMet asserts that the status of all given conditions is "True".
// Any condition that is not listed in parameter 'gates' is ignored.
// It returns a slice of unmet conditions for reporting.
// If all conditions are met, the returned slice is empty.
func FindUnmetStatusConditions(conditions v1beta1.Conditions, gates []v1beta1.ConditionType) []v1beta1.ConditionType {
	if gates == nil {
		return nil
	}

	if len(conditions) == 0 {
		return gates
	}

	unmet := make([]v1beta1.ConditionType, 0)
	for _, t := range gates {
		found := false
		for _, c := range conditions {
			if c.Type == t {
				found = true
				if c.Status != v1.ConditionTrue {
					unmet = append(unmet, t)
				}
				break
			}
		}
		if !found {
			unmet = append(unmet, t)
		}
	}
	return unmet
}
