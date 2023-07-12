package privateendpoints_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestPrivateendpoints(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Privateendpoints Suite")
}
