.PHONY: build install test clean doctor

# Build roji binary to ./bin/roji
build:
	go build -o bin/roji ./cmd/roji

# Install to $GOPATH/bin (or $HOME/go/bin)
install:
	go install ./cmd/roji

# Run all tests
test:
	go test ./...

# Clean build artifacts
clean:
	rm -rf bin/

# Build and run doctor (quick check)
doctor: build
	./bin/roji doctor
