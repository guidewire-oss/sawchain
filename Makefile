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

.PHONY: security-patch
security-patch:
	@test -n "$(MOD)" || (echo "Usage: make security-patch MOD=<module> VER=<version>"; exit 1)
	@test -n "$(VER)" || (echo "Usage: make security-patch MOD=<module> VER=<version>"; exit 1)
	go get $(MOD)@$(VER)
	$(MAKE) init test
	$(MAKE) cluster-up
	$(MAKE) bump-examples test-examples; result=$$?; $(MAKE) cluster-down; exit $$result

.PHONY: bump-examples
bump-examples:
	@test -n "$(MOD)" || (echo "Usage: make bump-examples MOD=<module> VER=<version>"; exit 1)
	@test -n "$(VER)" || (echo "Usage: make bump-examples MOD=<module> VER=<version>"; exit 1)
	@for dir in $(EXAMPLES); do \
		echo "==> Bumping $$dir"; \
		if [ -f "$$dir/Makefile" ]; then \
			(cd "$$dir" && go get $(MOD)@$(VER) && $(MAKE) init); \
		else \
			(cd "$$dir" && go get $(MOD)@$(VER) && go mod tidy); \
		fi; \
	done

.PHONY: test-examples
test-examples:
	@for dir in $(EXAMPLES); do \
		echo "==> Testing $$dir"; \
		if [ -f "$$dir/Makefile" ]; then \
			$(MAKE) -C "$$dir" test; \
		else \
			(cd "$$dir" && go test -v ./...); \
		fi; \
	done

.PHONY: cluster-up
cluster-up:
	k3d cluster create $(CLUSTER_NAME)
	vela install --yes

.PHONY: cluster-down
cluster-down:
	k3d cluster delete $(CLUSTER_NAME)
