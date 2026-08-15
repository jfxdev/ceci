.PHONY: dev backend frontend test build docker-build seed

GOBIN := $(shell go env GOPATH)/bin
AIR := $(shell command -v air 2>/dev/null || echo $(GOBIN)/air)

dev:
	@if [ ! -x "$(AIR)" ]; then \
		echo "air not found, installing to $(GOBIN)..."; \
		go install github.com/air-verse/air@latest; \
	fi
	docker compose up -d postgres
	trap 'kill 0' EXIT; \
	( cd backend && "$(AIR)" ) & \
	( cd frontend && npm run dev -- --port 8111 ) & \
	wait

backend:
	cd backend && "$(AIR)"

frontend:
	cd frontend && npm run dev -- --port 8111

test:
	cd backend && go test ./... -cover
	cd frontend && npx vitest run --coverage

build:
	cd frontend && npm run build
	rm -rf backend/internal/web/dist
	cp -r frontend/dist backend/internal/web/dist
	cd backend && go build -o ../bin/leaflag ./cmd/leaflag

docker-build:
	docker build -t leaflag:latest .

seed: ## Cria ou promove admin@leaflag.local a administrador (EMAIL, PASSWORD e NAME são opcionais)
	cd backend && go run ./cmd/seed --email "$${EMAIL:-admin@leaflag.local}" --password "$${PASSWORD:-admin123}" --name "$${NAME:-Admin}"
