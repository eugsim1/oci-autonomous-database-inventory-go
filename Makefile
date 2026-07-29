BINARY := oci-adb-inventory

.PHONY: build test fmt vet clean

build:
	go build -trimpath -o bin/$(BINARY) ./cmd/oci-adb-inventory

test:
	go test ./...

fmt:
	gofmt -w cmd internal

vet:
	go vet ./...

clean:
	go clean
