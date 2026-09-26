.PHONY: fmt fmt-check vet test

GO_FILES := $(shell git ls-files '*.go')

fmt:
	gofmt -w $(GO_FILES)

fmt-check:
	@unformatted="$$(gofmt -l $(GO_FILES))"; \
	if [ -n "$$unformatted" ]; then \
		echo "Go files need formatting:" >&2; \
		echo "$$unformatted" >&2; \
		exit 1; \
	fi

vet:
	go vet ./...
	cd infra && go vet ./...

test: fmt-check vet
	go test -race -cover -v ./...
	cd infra && go test -race -cover -v ./...
