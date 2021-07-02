.PHONY: all install-tools generate check stylecheck test teste2e

# Install (code generation) tools in $GOBIN.
install-tools:
	grep _ ./pkg/internal/tools/tools.go | cut -d'"' -f2 | xargs go install

# Generate code (expects $GOBIN to be in PATH)
generate:
	go generate ./pkg/...

# Check code for issues.
check:
	go fmt ./pkg/... ./cmd/...
	go vet ./pkg/... ./cmd/...

# Check code for style issues.
stylecheck:
	golint ./pkg/... ./cmd/...

# Run unit tests.
test: check
	go test ./pkg/... ./cmd/... -coverprofile test.cover

# Run e2e tests.
# Tests that require resouces that are not in --resources are skipped.
# See test/e2e/tool_generate_test.go for more on --resources flag.
teste2e: check
	go test -v ./test/e2e/... -coverprofile e2e.cover -args --resources=k8s,keyvault

# Snapshot binary.
snapshot: check
	goreleaser build --snapshot --single-target --rm-dist

# Install binary in PATH.
install-linux:
	sudo cp dist/kubectl-tmplt_linux_amd64/kubectl-tmplt /usr/local/bin/
	#sudo ln -sfr /usr/local/bin/kubectl-tmplt-$(VERSION)-linux-amd64 /usr/local/bin/kubectl-tmplt
