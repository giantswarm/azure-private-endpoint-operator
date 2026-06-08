package util

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	capi "sigs.k8s.io/cluster-api/api/core/v1beta1"
)

var ()

func TestUtilities(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Controllers Suite")
}

var _ = DescribeTable("AreStatusConditionsMet",
	func(conditions capi.Conditions, gates []capi.ConditionType, want []capi.ConditionType) {
		unmet := FindUnmetStatusConditions(conditions, gates)
		Expect(unmet).To(HaveExactElements(want))
	},
	Entry("When conditions are met returns no unmet conditions",
		conditions("Foo", "True"), types("Foo"), types(),
	),
	Entry("When conditions are not met returns the unmet conditions",
		conditions("Foo", "False"), types("Foo"), types("Foo"),
	),
	Entry("When some conditions are met returns only the unmet conditions",
		conditions("Foo", "True", "Bar", "False"), types("Foo", "Bar"), types("Bar"),
	),
	Entry("When not all conditions must be met, those conditions are ignored and returns no unmet conditions",
		conditions("Foo", "True", "Bar", "True", "Baz", "False"), types("Foo", "Bar"), types(),
	),
	Entry("When desired condition is absent returns it as unmet condition",
		conditions("Foo", "True"), types("Bar"), types("Bar"),
	),
	Entry("When conditions are empty returns all conditions",
		conditions(), types("Foo"), types("Foo"),
	),
	Entry("When no desired condition types are given returns all unmet conditions",
		conditions(), types("Foo", "Bar"), types("Foo", "Bar"),
	),
	Entry("When desired condition types is nil returns nil",
		conditions("Foo", "True"), nil, nil,
	),
)

// conditions is a helper for compactly generating a [capi.Conditions].
// It expects arguments in pairs, where the first in each pair is the condition type,
// and the second is the condition status.
func conditions(pairs ...string) capi.Conditions {
	if len(pairs)%2 != 0 {
		panic("uneven amount of arguments given")
	}

	conditions := make(capi.Conditions, 0)
	for i := 0; i < len(pairs); i += 2 {
		conditions = append(conditions, capi.Condition{
			Type:   capi.ConditionType(pairs[i]),
			Status: v1.ConditionStatus(pairs[i+1]),
		})
	}
	return conditions
}

// types is a helper for compactly generating a [][capi.ConditionType].
// It simply typecasts the given strings as [capi.ConditionType].
func types(t ...string) []capi.ConditionType {
	ts := make([]capi.ConditionType, len(t))
	for i := range t {
		ts[i] = capi.ConditionType(t[i])
	}
	return ts
}
