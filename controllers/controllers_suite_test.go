package controllers_test

import (
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/zap/zapcore"
	"golang.org/x/tools/go/packages"
	"k8s.io/apimachinery/pkg/apis/meta/internalversion/scheme"
	capz "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	capi "sigs.k8s.io/cluster-api/api/core/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/envtest/komega"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func TestControllers(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Controllers Suite")
}

var (
	testEnv *envtest.Environment
)

var _ = BeforeSuite(func() {
	opts := zap.Options{
		DestWriter:  GinkgoWriter,
		Development: true,
		TimeEncoder: zapcore.RFC3339TimeEncoder,
	}
	logger := zap.New(zap.UseFlagOptions(&opts))
	log.SetLogger(logger)

	// Look up packages on-disk so that we can load the CRDs.
	capiModule, err := packages.Load(&packages.Config{Mode: packages.NeedModule}, "sigs.k8s.io/cluster-api")
	Expect(err).NotTo(HaveOccurred())
	capzModule, err := packages.Load(&packages.Config{Mode: packages.NeedModule}, "sigs.k8s.io/cluster-api-provider-azure")
	Expect(err).NotTo(HaveOccurred())

	By("bootstrapping test environment")
	kubeBuilderAssetsPath, err := envtest.SetupEnvtestDefaultBinaryAssetsDirectory()
	Expect(err).NotTo(HaveOccurred())

	testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{
			filepath.Join(capiModule[0].Module.Dir, "config", "crd", "bases"),
			filepath.Join(capiModule[0].Module.Dir, "controlplane", "kubeadm", "config", "crd", "bases"),
			filepath.Join(capzModule[0].Module.Dir, "config", "crd", "bases"),
		},
		ErrorIfCRDPathMissing: true,
		BinaryAssetsDirectory: kubeBuilderAssetsPath,
		DownloadBinaryAssets:  true,

		Scheme: scheme.Scheme,
	}

	cfg, err := testEnv.Start()
	Expect(err).ToNot(HaveOccurred())

	Expect(capi.AddToScheme(scheme.Scheme)).Should(Succeed())
	Expect(capz.AddToScheme(scheme.Scheme)).Should(Succeed())

	k8sClient, err := client.New(cfg, client.Options{})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	komega.SetClient(k8sClient)
})

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	if testEnv == nil {
		return
	}

	Expect(testEnv.Stop()).Should(Succeed())
})
