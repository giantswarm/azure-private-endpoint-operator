package testhelpers

import (
	"fmt"

	"github.com/onsi/gomega/types"
	"sigs.k8s.io/cluster-api/api/v1beta1"

	"github.com/giantswarm/azure-private-endpoint-operator/pkg/util"
)

// MeetConditions is a Gomega matcher that wraps around [util.AreStatusConditionsMet].
func MeetConditions(gates ...v1beta1.ConditionType) types.GomegaMatcher {
	return &matchStatusCondition{gates: gates}
}

type matchStatusCondition struct {
	gates []v1beta1.ConditionType
	unmet []v1beta1.ConditionType
}

func (m *matchStatusCondition) Match(actual any) (success bool, err error) {
	conditions, ok := actual.(v1beta1.Conditions)
	if !ok {
		return false, fmt.Errorf("actual should be of type v1beta1.Conditions")
	}

	m.unmet = util.AreStatusConditionsMet(conditions, m.gates)
	if len(m.unmet) > 0 {
		return false, nil
	}
	return true, nil
}

func (m *matchStatusCondition) FailureMessage(actual any) (message string) {
	return fmt.Sprintf("expected status conditions\n\t%#v\nto meet\n\t%#v\n", actual, m.unmet)
}

func (m *matchStatusCondition) NegatedFailureMessage(actual any) (message string) {
	return fmt.Sprintf("expected status conditions\n\t%#v\nto not match\n\t%#v\n", actual, m.gates)
}
