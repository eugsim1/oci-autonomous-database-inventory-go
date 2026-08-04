GO := go
GOFMT := gofmt
BINARY := oci-adb-inventory
BINARY_PATH := bin/$(BINARY)
CGO_ENABLED ?= 0
LDFLAGS ?= -s -w

.PHONY: all help deps build rebuild check test fmt vet clean

all: build

help:
	@printf '%s\n' \
		'make deps     Download modules from go.mod/go.sum' \
		'make test     Run all Go tests' \
		'make vet      Run Go static analysis' \
		'make check    Run tests and vet' \
		'make fmt      Format Go files under cmd and internal' \
		'make build    Build bin/oci-adb-inventory' \
		'make clean    Remove only bin/oci-adb-inventory' \
		'make rebuild  Remove the old binary and build a new one'

deps:
	$(GO) mod download

build:
	@mkdir -p bin
	CGO_ENABLED=$(CGO_ENABLED) $(GO) build -trimpath -ldflags="$(LDFLAGS)" -o $(BINARY_PATH) ./cmd/oci-adb-inventory

rebuild: clean build

check: test vet

test:
	$(GO) test ./...

fmt:
	$(GOFMT) -w cmd internal

vet:
	$(GO) vet ./...

clean:
	$(RM) -- $(BINARY_PATH)
