GO  ?= go
BIN := tmp/minimal

.PHONY: all build test vet fmt example run clean
all: vet test build

build:
	$(GO) build ./...

test:
	$(GO) test ./...

vet:
	$(GO) vet ./...

fmt:
	gofmt -w kit examples

fmt-check:
	test -z "$$(gofmt -l kit examples)" || (gofmt -l kit examples; exit 1)

example:
	$(GO) build -o $(BIN) ./examples/minimal

run: example
	./$(BIN) -port 8080 -data-dir tmp/rundata -log-console

clean:
	find tmp -mindepth 1 ! -name .gitkeep -delete
