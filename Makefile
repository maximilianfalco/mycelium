.PHONY: build test lint run generate clean dev dev-stop db db-stop api frontend

BINARY=myc

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

dev: build db
	@echo "starting mycelium..."
	@echo "api:      http://localhost:8080"
	@echo "frontend: http://localhost:3000"
	@echo "press ctrl+c to stop"
	@echo ""
	@trap 'kill 0' EXIT; \
		./$(BINARY) serve & \
		cd frontend && npm run dev & \
		wait
