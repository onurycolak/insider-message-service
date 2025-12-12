.PHONY: build run clean deps swagger docker-logs lint fmt migrate-up create-message scheduler-status scheduler-start scheduler-stop get-sent get-stats health seed help stop docker-up docker-build docker-restart test

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
BINARY_NAME=insider-message-service
MAIN_PATH=.

# Dev API keys (must match .env values)
MESSAGES_API_KEY=dev-messages-key
SCHEDULER_API_KEY=dev-scheduler-key

# Build Docker images and start containers
build:
	@echo "Building Docker images..."
	docker compose build
	@echo "Starting containers..."
	docker compose up -d
	@echo "Containers started - API at http://localhost:8080"

# Run containers (alias for build after build)
run:
	@echo "Starting containers..."
	docker compose up -d
	@echo "Containers started - API at http://localhost:8080"

# Download dependencies
deps:
	@echo "Downloading dependencies..."
	$(GOMOD) download
	$(GOMOD) tidy
	@echo "Dependencies ready"

# Generate swagger documentation
swagger:
	@echo "Generating Swagger documentation..."
	swag init -g main.go -o docs
	@echo "Swagger docs generated"

# Seed database using standalone seeder
seed:
	@echo "Seeding database..."
	$(GOCMD) run ./db/seed
	@echo "Seed complete"

# Start Docker containers
docker-up:
	@echo "Starting Docker containers..."
	docker compose up -d
	@echo "Containers started"
	@echo "API available at: http://localhost:8080"
	@echo "Swagger UI at: http://localhost:8080/swagger/index.html"

# Stop containers
stop:
	@echo "Stopping containers..."
	docker compose down
	@echo "Containers stopped"

# Clean containers, volumes, and images
clean:
	@echo "Cleaning containers, volumes, and images..."
	docker compose down -v --rmi all --remove-orphans
	@echo "Cleaned"

# Build Docker image
docker-build:
	@echo "Building Docker image..."
	docker compose build
	@echo "Docker image built"

# View Docker logs
docker-logs:
	docker compose logs -f app

# Rebuild and restart
docker-restart: stop docker-build docker-up

# Run linter
lint:
	@echo "Running linter..."
	golangci-lint run ./...
	@echo "Lint complete"

# Format code
fmt:
	@echo "Formatting code..."
	gofmt -s -w .
	@echo "Format complete"

# Run tests
test:
	@echo "Running Go tests..."
	go test ./...

# Database migrations (manual)
migrate-up:
	@echo "Running migrations..."
	@echo "Migrations are run automatically on app start"

# Create a new message via API
create-message:
	@echo "Creating test message..."
	curl -X POST http://localhost:8080/api/v1/messages \
		-H "Content-Type: application/json" \
		-H "x-ins-auth-key: $(MESSAGES_API_KEY)" \
		-d '{"content": "Test message from Makefile", "phoneNumber": "+905551234567"}'

# Get scheduler status
scheduler-status:
	@echo "Getting scheduler status..."
	curl -s http://localhost:8080/api/v1/scheduler/status \
		-H "x-ins-auth-key: $(SCHEDULER_API_KEY)" | jq

# Start scheduler
scheduler-start:
	@echo "Starting scheduler..."
	curl -X POST http://localhost:8080/api/v1/scheduler/start \
		-H "x-ins-auth-key: $(SCHEDULER_API_KEY)"

# Stop scheduler
scheduler-stop:
	@echo "Stopping scheduler..."
	curl -X POST http://localhost:8080/api/v1/scheduler/stop \
		-H "x-ins-auth-key: $(SCHEDULER_API_KEY)"

# Get sent messages
get-sent:
	@echo "Getting sent messages..."
	curl -s http://localhost:8080/api/v1/messages/sent \
		-H "x-ins-auth-key: $(MESSAGES_API_KEY)" | jq

# Get message stats
get-stats:
	@echo "Getting message stats..."
	curl -s http://localhost:8080/api/v1/messages/stats \
		-H "x-ins-auth-key: $(MESSAGES_API_KEY)" | jq

# Health check
health:
	@echo "Health check..."
	curl -s http://localhost:8080/health | jq

# Help
help:
	@echo "Insider Message Service - Available Commands:"
	@echo ""
	@echo "Build & Run:"
	@echo "  make build          - Build Docker images and start containers"
	@echo "  make run            - Start containers (after build)"
	@echo "  make stop           - Stop containers"
	@echo "  make clean          - Remove containers, volumes, and images"
	@echo ""
	@echo "Development:"
	@echo "  make deps           - Download Go dependencies"
	@echo "  make swagger        - Generate Swagger documentation"
	@echo "  make lint           - Run linter"
	@echo "  make fmt            - Format code"
	@echo "  make test           - Run Go tests"
	@echo "  make seed           - Seed the database (optional)"
	@echo ""
	@echo "Docker & Logs:"
	@echo "  make docker-logs    - View application logs"
	@echo "  make docker-up      - Start Docker containers"
	@echo "  make docker-restart - Rebuild and restart containers"
	@echo ""
	@echo "API Commands:"
	@echo "  make health           - Check API health"
	@echo "  make scheduler-status - Get scheduler status"
	@echo "  make scheduler-start  - Start the scheduler"
	@echo "  make scheduler-stop   - Stop the scheduler"
	@echo "  make get-sent         - Get sent messages"
	@echo "  make get-stats        - Get message statistics"
	@echo "  make create-message   - Create a test message"
