package options_test

import (
	"os"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/guidewire-oss/sawchain/internal/testutil"
)

const templateContent = `
	    ---
	    # Empty document
	    ---

	    ---
	    apiVersion: v1
	    kind: ConfigMap
	    metadata:
	      name: test-config
	      namespace: default
	    data:
	      key1: value1
	      key2: value2
	      key3: value3

	    ---
	    # Document with just comments
	    # and blank lines

	    ---
	    apiVersion: v1
	    kind: Service
	    metadata:
	      name: test-service
	    spec:
	      selector:
	        app: test
	      ports:
	        - port: 80
	          targetPort: 8080
`

const sanitizedTemplateContent = `apiVersion: v1
kind: ConfigMap
metadata:
  name: test-config
  namespace: default
data:
  key1: value1
  key2: value2
  key3: value3
---
apiVersion: v1
kind: Service
metadata:
  name: test-service
spec:
  selector:
    app: test
  ports:
    - port: 80
      targetPort: 8080`

var templateFilePath = testutil.CreateTempFile("template-*.yaml", templateContent)

func TestOptions(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Options Suite")
}

var _ = AfterSuite(func() {
	Expect(os.Remove(templateFilePath)).To(Succeed())
})
