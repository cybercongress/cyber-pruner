all: install

build:
	@echo "Building cyber-pruner"
	@go build -o build/cyber-pruner main.go

install:
	@echo "Installing cyber-pruner"
	@go install ./...

clean:
	rm -rf build

.PHONY: all build install clean
