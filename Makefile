.PHONY: build run test test-race test-cover lint swagger godoc docker-up docker-down clean install-tools help

BIN        := sftpgo-manager
COVER_FILE := coverage.out

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'

build: ## Build the binary
	go build -o $(BIN) .

run: build ## Build and run the server
	./$(BIN)

test: ## Run all tests
	go test ./... -count=1

test-race: ## Run tests with race detector
	go test ./... -count=1 -race

test-cover: ## Run tests with coverage report
	go test ./... -count=1 -coverprofile=$(COVER_FILE)
	go tool cover -func=$(COVER_FILE)
	@echo "\nOpen HTML report: make test-cover-html"

test-cover-html: test-cover ## Open coverage report in browser
	go tool cover -html=$(COVER_FILE)

lint: ## Run golangci-lint
	golangci-lint run ./...

swagger: ## Regenerate Swagger docs
	swag init
	@echo "Swagger docs regenerated in docs/"

godoc: ## Serve godoc on http://localhost:6060
	@echo "Opening http://localhost:6060/pkg/sftpgo-manager/"
	@echo "Press Ctrl+C to stop"
	godoc -http=:6060

docker-up: ## Start full stack (SFTPGo + MinIO + backend)
	docker compose up -d --build

docker-down: ## Stop and remove containers
	docker compose down

docker-logs: ## Tail logs from all services
	docker compose logs -f

clean: ## Remove build artifacts
	rm -f $(BIN) $(COVER_FILE)

install-tools: ## Install dev tools (swag, godoc, golangci-lint)
	go install github.com/swaggo/swag/cmd/swag@latest
	go install golang.org/x/tools/cmd/godoc@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
