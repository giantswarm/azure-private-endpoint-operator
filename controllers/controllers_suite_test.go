package controllers_test

import (
	"fmt"
	"go/build"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/zap/zapcore"
	"golang.org/x/tools/go/packages"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func TestControllers(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Controllers Suite")
}

var _ = BeforeSuite(func() {
	opts := zap.Options{
		DestWriter:  GinkgoWriter,
		Development: true,
		TimeEncoder: zapcore.RFC3339TimeEncoder,
	}
	logger := zap.New(zap.UseFlagOptions(&opts))
	log.SetLogger(logger)

	// We need to calculate the cluster-api version to load the CRDs from the right path
	capiModule, err := packages.Load(&packages.Config{Mode: packages.NeedModule}, "sigs.k8s.io/cluster-api")
	Expect(err).NotTo(HaveOccurred())
	// We need to calculate the cluster-api-provider-aws version to load the CRDs from the right path
	capzModule, err := packages.Load(&packages.Config{Mode: packages.NeedModule}, "sigs.k8s.io/cluster-api-provider-azure")
	Expect(err).NotTo(HaveOccurred())

	By("bootstrapping test environment")
	path, err := envtest.SetupEnvtestDefaultBinaryAssetsDirectory()
	Expect(err).NotTo(HaveOccurred())

	env := &envtest.Environment{
		CRDDirectoryPaths: []string{
			filepath.Join(build.Default.GOPATH, "pkg", "mod", "sigs.k8s.io", fmt.Sprintf("cluster-api@%s", capiModule[0].Module.Version), "config", "crd", "bases"),
			filepath.Join(build.Default.GOPATH, "pkg", "mod", "sigs.k8s.io", fmt.Sprintf("cluster-api@%s", capiModule[0].Module.Version), "controlplane", "kubeadm", "config", "crd", "bases"),
			filepath.Join(build.Default.GOPATH, "pkg", "mod", "sigs.k8s.io", fmt.Sprintf("cluster-api-provider-azure@%s", capzModule[0].Module.Version), "config", "crd", "bases"),
		},
		ErrorIfCRDPathMissing: true,
	}
})
