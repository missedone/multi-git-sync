REPO = 
IMAGE_TAG ?= dev
IMAGE := multi-git-sync

.PHONY: lint test build snapshot release

mod:
	go mod download
	go mod tidy

lint: mod
	gofmt -l -s -w .
	golangci-lint run

test: lint
	go test -coverprofile=coverage.txt -test.v ./...

ci: test
	gotestsum --junitfile report.xml --format testname
	gocover-cobertura < coverage.txt > coverage.xml

build:
	goreleaser build --snapshot --clean

snapshot:
	IMAGE_TAG=$(IMAGE_TAG) goreleaser release --skip=publish --snapshot --clean

release:
	IMAGE_TAG=$(IMAGE_TAG) goreleaser release --skip=publish --clean

push:
	docker push $(REPO)$(IMAGE):$(IMAGE_TAG)

clean:
	rm -rf dist
