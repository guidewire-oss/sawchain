package example

import (
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"

	"github.com/guidewire-oss/sawchain"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("nginx chart", func() {
	var (
		chartPath = filepath.Join("charts", "nginx")

		serviceAccount *corev1.ServiceAccount
		deployment     *appsv1.Deployment
		service        *corev1.Service
		ingress        *networkingv1.Ingress
		hpa            *autoscalingv2.HorizontalPodAutoscaler

		sc *sawchain.Sawchain
	)

	BeforeEach(func() {
		// Initialize Sawchain with fake client
		sc = sawchain.New(GinkgoTB(), fake.NewClientBuilder().Build())
		Expect(sc).NotTo(BeNil())
	})

	Context("with defaults", func() {
		It("renders core resources", func() {
			// Run helm template
			output, err := runHelmTemplate("defaults", chartPath, "")
			Expect(err).NotTo(HaveOccurred())
			Expect(output).NotTo(BeEmpty())

			// Unmarshal objects
			objs := sc.RenderMultiple(output)
			Expect(objs).To(HaveLen(4))

			// Check ServiceAccount
			serviceAccount = findObjectByType[*corev1.ServiceAccount](objs)
			Expect(serviceAccount).To(sc.MatchYAML(filepath.Join("yaml", "defaults", "serviceaccount.yaml")))

			// Check Deployment
			deployment = findObjectByType[*appsv1.Deployment](objs)
			Expect(deployment).To(sc.MatchYAML(filepath.Join("yaml", "defaults", "deployment.yaml")))

			// Check Service
			service = findObjectByType[*corev1.Service](objs)
			Expect(service).To(sc.MatchYAML(filepath.Join("yaml", "defaults", "service.yaml")))

			// The 4th resource is the 'test-connection' Pod which we don't care much about
		})
	})

	Context("with deployment overrides", func() {
		It("renders modified deployment", func() {
			// Run helm template
			valuesPath := filepath.Join("yaml", "overrides", "values.yaml")
			output, err := runHelmTemplate("overrides", chartPath, valuesPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(output).NotTo(BeEmpty())

			// Unmarshal objects
			objs := sc.RenderMultiple(output)
			Expect(objs).To(HaveLen(4))

			// Check Deployment
			deployment = findObjectByType[*appsv1.Deployment](objs)
			Expect(deployment).To(sc.MatchYAML(filepath.Join("yaml", "overrides", "deployment.yaml")))
		})
	})

	Context("with ingress enabled", func() {
		It("renders ingress resources", func() {
			// Run helm template
			valuesPath := filepath.Join("yaml", "ingress", "values.yaml")
			output, err := runHelmTemplate("ingress", chartPath, valuesPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(output).NotTo(BeEmpty())

			// Unmarshal objects
			objs := sc.RenderMultiple(output)
			Expect(objs).To(HaveLen(5))

			// Check Ingress
			ingress = findObjectByType[*networkingv1.Ingress](objs)
			Expect(ingress).To(sc.MatchYAML(filepath.Join("yaml", "ingress", "ingress.yaml")))
		})
	})

	Context("with autoscaling enabled", func() {
		It("renders autoscaling resources", func() {
			// Run helm template
			valuesPath := filepath.Join("yaml", "autoscaling", "values.yaml")
			output, err := runHelmTemplate("autoscaling", chartPath, valuesPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(output).NotTo(BeEmpty())

			// Unmarshal objects
			objs := sc.RenderMultiple(output)
			Expect(objs).To(HaveLen(5))

			// Check HPA
			hpa = findObjectByType[*autoscalingv2.HorizontalPodAutoscaler](objs)
			Expect(hpa).To(sc.MatchYAML(filepath.Join("yaml", "autoscaling", "hpa.yaml")))
		})
	})
})

// HELPERS

// runHelmTemplate runs `helm template` with the given release name, chart path, and values path.
// It returns the rendered YAML output or an error if the command fails.
func runHelmTemplate(releaseName, chartPath, valuesPath string) (string, error) {
	args := []string{"template", releaseName, chartPath}
	if valuesPath != "" {
		args = append(args, "--values", valuesPath)
	}

	var stdout, stderr bytes.Buffer
	cmd := exec.Command("helm", args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to run helm template: %w\nstderr: %s", err, stderr.String())
	}

	return stdout.String(), nil
}

// findObjectByType returns the first object of type T found in the given slice.
func findObjectByType[T client.Object](objs []client.Object) (t T) {
	var ok bool
	for _, obj := range objs {
		if t, ok = obj.(T); ok {
			return
		}
	}
	return
}
