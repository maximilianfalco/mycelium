.PHONY: build test lint run generate clean dev dev-stop db db-stop api frontend \
       docker-build docker-up docker-down docker-logs docker-rebuild

BINARY=myc
AIR=$(shell go env GOPATH)/bin/air

build:
	go build -o $(BINARY) ./cmd/myc

test: db
	@until docker compose exec -T db pg_isready -U mycelium -q 2>/dev/null; do sleep 0.5; done
	@docker compose exec -T db psql -U mycelium -d mycelium -f /docker-entrypoint-initdb.d/001-schema.sql -q 2>/dev/null || true
	go test ./... -v

lint:
	go vet ./...

run: build
	./$(BINARY)

generate:
	@echo "TODO: oapi-codegen + openapi-typescript"

clean:
	rm -f $(BINARY)
	go clean -testcache

# --- Full stack ---

db:
	docker compose up -d

db-stop:
	docker compose down

api: build
	./$(BINARY) serve

frontend:
	cd frontend && npm run dev

dev: db
	@echo "starting mycelium..."
	@echo "api:      http://localhost:8080 (live reload)"
	@echo "frontend: http://localhost:3773"
	@echo "press ctrl+c to stop"
	@echo ""
	@trap 'kill 0' EXIT; \
		$(AIR) & \
		cd frontend && npm run dev & \
		wait

# --- Docker (full stack, backgrounded) ---

docker-build:
	docker compose build

docker-up:
	docker compose up -d
	@echo ""
	@echo "mycelium is running:"
	@echo "  frontend: http://localhost:3773"
	@echo "  api:      http://localhost:8080"
	@echo "  pgadmin:  http://localhost:5050"
	@echo ""
	@echo "use 'make docker-logs' to tail logs"
	@echo "use 'make docker-down' to stop"

docker-down:
	docker compose down

docker-logs:
	docker compose logs -f

docker-rebuild: docker-down
	docker compose build --no-cache
	docker compose up -d
