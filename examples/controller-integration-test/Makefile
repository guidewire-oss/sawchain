.PHONY: init
init:
	go mod tidy
	go install sigs.k8s.io/controller-tools/cmd/controller-gen@v0.18.0
	go install sigs.k8s.io/controller-runtime/tools/setup-envtest@v0.0.0-20250505003155-b6c5897febe5

.PHONY: generate
generate:
	controller-gen object paths=./...

.PHONY: manifests
manifests:
	controller-gen crd paths=./...

.PHONY: test
test:
	KUBEBUILDER_ASSETS="$(shell setup-envtest use -p path 1.33)" go test ./... -v
