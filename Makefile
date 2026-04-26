BIN  := reliure
OUT  := bin/$(BIN)
PKG  := ./cmd/reliure
LDFLAGS := -s -w -X main.version=$(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

.PHONY: build build-static test vet tidy install-local clean

build:
	@mkdir -p bin
	go build -trimpath -ldflags="$(LDFLAGS)" -o $(OUT) $(PKG)
	@echo "  → $(OUT)"

build-static:
	@mkdir -p bin
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
		go build -trimpath -ldflags="$(LDFLAGS)" -o $(OUT) $(PKG)
	@echo "  → $(OUT)"

test:
	go test ./...

vet:
	go vet ./...

tidy:
	go mod tidy

install-local: build
	install -Dm755 $(OUT) $(HOME)/.local/bin/$(BIN)

clean:
	rm -rf bin/
