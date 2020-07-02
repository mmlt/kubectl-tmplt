.PHONY: all install-tools generate test bin check

VERSION?=v0.2.0-alpha

# CI/CD target.
all: install-tools generate bin

# Install (code generation) tools.
install-tools:
	grep _ ./pkg/internal/tools/tools.go | cut -d'"' -f2 | xargs go install

# Generate code (expects $GOBIN to be in PATH)
generate:
	go generate ./pkg/...

# Run tests.
test: check
	go test ./pkg/... ./cmd/... -coverprofile test.cover

teste2e: check
	go test ./test/e2e/... -coverprofile e2e.cover

# Create binaries.
bin: check
	GOOS=linux GOARCH=amd64 go build -ldflags "-X main.Version=$(VERSION)" -o bin/kubectl-tmplt-$(VERSION)-linux-amd64 github.com/mmlt/kubectl-tmplt/cmd/plugin
	GOOS=windows GOARCH=amd64 go build -ldflags "-X main.Version=$(VERSION)" -o bin/kubectl-tmplt-$(VERSION)-windows-amd64.exe github.com/mmlt/kubectl-tmplt/cmd/plugin
	GOOS=darwin GOARCH=amd64 go build -ldflags "-X main.Version=$(VERSION)" -o bin/kubectl-tmplt-$(VERSION)-darwin-amd64 github.com/mmlt/kubectl-tmplt/cmd/plugin

# Check code for issues.
check:
	go fmt ./pkg/... ./cmd/...
	go vet ./pkg/... ./cmd/...

# Check code for style issues.
stylecheck:
	golint ./pkg/... ./cmd/...

# Install binary in PATH.
install-linux:
	sudo cp bin/kubectl-tmplt-$(VERSION)-linux-amd64 /usr/local/bin/
	sudo ln -sfr /usr/local/bin/kubectl-tmplt-$(VERSION)-linux-amd64 /usr/local/bin/kubectl-tmplt

