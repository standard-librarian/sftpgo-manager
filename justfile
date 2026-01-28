bin := "sftpgo-manager"
cover_file := "coverage.out"

# List available recipes
default:
    @just --list

# Build the binary
build:
    go build -o {{bin}} .

# Build and run the server
run: build
    ./{{bin}}

# Run all tests
test:
    go test ./... -count=1

# Run tests with race detector
test-race:
    go test ./... -count=1 -race

# Run tests verbose
test-verbose:
    go test ./... -count=1 -v

# Run tests with coverage report
test-cover:
    go test ./... -count=1 -coverprofile={{cover_file}}
    go tool cover -func={{cover_file}}

# Open coverage report in browser
test-cover-html: test-cover
    go tool cover -html={{cover_file}}

# Run golangci-lint
lint:
    golangci-lint run ./...

# Regenerate Swagger docs
swagger:
    swag init
    @echo "Swagger docs regenerated in docs/"

# Serve godoc on http://localhost:6060
godoc:
    @echo "Opening http://localhost:6060/pkg/sftpgo-manager/"
    @echo "Press Ctrl+C to stop"
    godoc -http=:6060

# Start full stack (SFTPGo + MinIO + backend)
docker-up:
    docker compose up -d --build

# Stop and remove containers
docker-down:
    docker compose down

# Tail logs from all services
docker-logs:
    docker compose logs -f

# Remove build artifacts
clean:
    rm -f {{bin}} {{cover_file}}

# Install dev tools (swag, godoc, golangci-lint)
install-tools:
    go install github.com/swaggo/swag/cmd/swag@latest
    go install golang.org/x/tools/cmd/godoc@latest
    go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Run build + test + lint (CI check)
ci: build test-race lint
