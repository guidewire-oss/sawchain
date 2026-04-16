CR_RELEASE := $(shell sed -n 's/.*sigs.k8s.io\/controller-runtime v\([0-9]*\.[0-9]*\).*/\1/p' go.mod)

.PHONY: init
init:
	go mod tidy
	go install sigs.k8s.io/controller-tools/cmd/controller-gen@latest
	go install sigs.k8s.io/controller-runtime/tools/setup-envtest@release-$(CR_RELEASE)

.PHONY: generate
generate:
	controller-gen object paths=./...

.PHONY: manifests
manifests:
	controller-gen crd paths=./...

.PHONY: test
test:
	KUBEBUILDER_ASSETS="$(shell setup-envtest use -p path)" go test ./... -v
