.PHONY: build test lint run generate clean dev dev-stop db db-stop api frontend

BINARY=myc
AIR=$(shell go env GOPATH)/bin/air

build:
	go build -o $(BINARY) ./cmd/myc

test:
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
