.DEFAULT_GOAL := default

.PHONY: default
default: build

.PHONY: update
update:
	go get -u ./...
	go mod tidy -v

.PHONY: cleancode
cleancode:
	go fmt ./...
	go vet ./...

.PHONY: build
build:
	go build -o ffuf

.PHONY: lint
lint:
	"$$(go env GOPATH)/bin/golangci-lint" run ./...
	go mod tidy

.PHONY: lint-update
lint-update:
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $$(go env GOPATH)/bin
	$$(go env GOPATH)/bin/golangci-lint --version

.PHONY: test
test:
	go test ./...
