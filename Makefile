.PHONY: init
init:
	go mod tidy
	go install github.com/princjef/gomarkdoc/cmd/gomarkdoc@v1.1.0

.PHONY: test
test:
	go test ./ ./internal/... -coverprofile=coverage.txt

.PHONY: debug
debug:
	dlv test $${PACKAGE:-./} --listen=:40000 --headless=true --api-version=2

.PHONY: docs
docs:
	gomarkdoc --repository.url "https://github.com/guidewire-oss/sawchain" ./ \
	| awk 'BEGIN { RS=""; ORS="\n\n" } { gsub(/```\n/, "```go\n"); print }' \
	> docs/api-reference.md

EXAMPLES := $(wildcard examples/*/)
CLUSTER_NAME := sawchain-test

.PHONY: bump-all
bump-all:
	@test -n "$(MODS)" || (echo "Usage: make bump-all MODS=\"mod1@v1 mod2@v2 ...\""; exit 1)
	go get $(MODS)
	go mod tidy
	@for dir in $(EXAMPLES); do \
		echo "==> Tidying $$dir"; \
		if [ -f "$$dir/Makefile" ]; then \
			$(MAKE) -C "$$dir" init || exit 1; \
		else \
			(cd "$$dir" && go mod tidy) || exit 1; \
		fi; \
	done

.PHONY: test-all
test-all:
	$(MAKE) test
	$(MAKE) cluster-up
	$(MAKE) test-examples; result=$$?; $(MAKE) cluster-down; exit $$result

.PHONY: test-examples
test-examples:
	@for dir in $(EXAMPLES); do \
		echo "==> Testing $$dir"; \
		if [ -f "$$dir/Makefile" ]; then \
			$(MAKE) -C "$$dir" test || exit 1; \
		else \
			(cd "$$dir" && go test -v ./...) || exit 1; \
		fi; \
	done

.PHONY: cluster-up
cluster-up:
	k3d cluster create $(CLUSTER_NAME)
	vela install --yes

.PHONY: cluster-down
cluster-down:
	k3d cluster delete $(CLUSTER_NAME)
