
.PHONY: test
test:
	go test ./pkg/... ./cmd/... -coverprofile cover.out

.PHONY: bin
bin: fmt vet
	go build -o bin/kubectl-tmplt github.com/mmlt/kubectl-tmplt/cmd/plugin

.PHONY: fmt
fmt:
	go fmt ./pkg/... ./cmd/...

.PHONY: vet
vet:
	go vet ./pkg/... ./cmd/...

.PHONY: kubernetes-deps
kubernetes-deps:
	go get k8s.io/client-go@v0.17.3
	go get k8s.io/apimachinery@v0.17.3
	go get k8s.io/cli-runtime@v0.17.3

.PHONY: setup
setup:
	make -C setup