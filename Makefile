.PHONY: init
init:
	go mod tidy
	go install github.com/princjef/gomarkdoc/cmd/gomarkdoc@latest

.PHONY: test
test:
	go test ./ ./internal/... -coverprofile=coverage.txt

.PHONY: debug
debug:
	dlv test $${PACKAGE:-./} --listen=:40000 --headless=true --api-version=2

.PHONY: docs
docs:
	gomarkdoc ./ \
	| awk 'BEGIN { RS=""; ORS="\n\n" } { gsub(/```\n/, "```go\n"); print }' \
	> docs/REFERENCE.md
