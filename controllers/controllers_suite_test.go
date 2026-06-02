package controllers_test

import (
	"context"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/zap/zapcore"
	"golang.org/x/tools/go/packages"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/uuid"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	capz "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	kcp "sigs.k8s.io/cluster-api/api/controlplane/kubeadm/v1beta2"
	capiv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	capi "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/envtest/komega"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/giantswarm/azure-private-endpoint-operator/pkg/testhelpers"
)

func TestControllers(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Controllers Suite")
}

var (
	testEnv   *envtest.Environment
	k8sClient client.Client
	namespace string
	scheme    = runtime.NewScheme()
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

	Expect(clientgoscheme.AddToScheme(scheme)).Should(Succeed())
	Expect(capi.AddToScheme(scheme)).Should(Succeed())
	Expect(capiv1beta1.AddToScheme(scheme)).Should(Succeed())
	Expect(kcp.AddToScheme(scheme)).Should(Succeed())
	Expect(capz.AddToScheme(scheme)).Should(Succeed())
	testhelpers.SetScheme(scheme)

	testEnv = &envtest.Environment{
		Scheme: scheme,
		CRDDirectoryPaths: []string{
			// CAPI Core (Cluster, ...)
			filepath.Join(capiModule[0].Module.Dir, "config", "crd", "bases"),
			// CAPI ControlPlane (KubeadmControlPlane, ...)
			filepath.Join(capiModule[0].Module.Dir, "controlplane", "kubeadm", "config", "crd", "bases"),
			// CAPZ Core (AzureCluster, ...)
			filepath.Join(capzModule[0].Module.Dir, "config", "crd", "bases"),
			// Additional CRDs that we depend on (Crossplane, ...)
			filepath.Join("..", "tests", "testdata", "crds"),
		},
		ErrorIfCRDPathMissing: true,
		BinaryAssetsDirectory: kubeBuilderAssetsPath,
		DownloadBinaryAssets:  true,
	}

	cfg, err := testEnv.Start()
	Expect(err).ToNot(HaveOccurred())

	k8sClient, err = client.New(cfg, client.Options{
		Scheme: scheme,
	})
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

var _ = BeforeEach(func() {
	namespace = string(uuid.NewUUID())
	namespaceObj := corev1.Namespace{}
	namespaceObj.Name = namespace
	Expect(k8sClient.Create(context.Background(), &namespaceObj)).To(Succeed())
})

var _ = AfterEach(func() {
	namespaceObj := corev1.Namespace{}
	namespaceObj.Name = namespace
	Expect(k8sClient.Delete(context.Background(), &namespaceObj)).To(Succeed())
})
