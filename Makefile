.PHONY: init
init:
	go mod tidy

.PHONY: test
test: init
	go test ./ ./internal/... -coverprofile=coverage.txt

.PHONY: debug
debug: init
	dlv test $${PACKAGE:-./} --listen=:40000 --headless=true --api-version=2

.PHONY: docs
docs:
	gomarkdoc ./ \
	| awk 'BEGIN { RS=""; ORS="\n\n" } { gsub(/```\n/, "```go\n"); print }' \
	> docs/REFERENCE.md
