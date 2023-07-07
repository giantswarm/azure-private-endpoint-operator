package privatelinks_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestPrivateLinks(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "PrivateLinks Suite")
}
