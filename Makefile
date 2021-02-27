.DEFAULT_GOAL := default

.PHONY: default
default: build

.PHONY: docker-update
docker-update:
	docker pull golang:latest
	docker pull alpine:latest

.PHONY: docker-build
docker-build: docker-update
	docker build -t ffuf:dev .

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

.PHONY: docker-lint
docker-lint:
	docker pull golangci/golangci-lint:latest
	docker run --rm -v $$(pwd):/app -w /app golangci/golangci-lint:latest golangci-lint run

.PHONY: test
test:
	go test ./...
