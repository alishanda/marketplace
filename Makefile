.PHONY: up down run test race fallback timeout tidy

up:
	docker compose up -d postgres
	@until docker compose exec -T postgres pg_isready -U marketplace >/dev/null 2>&1; do sleep 1; done

down:
	docker compose down

run: up
	DATABASE_URL=postgres://marketplace:marketplace@localhost:5432/marketplace?sslmode=disable go run ./cmd/api

test: up
	DATABASE_URL=postgres://marketplace:marketplace@localhost:5432/marketplace?sslmode=disable go test ./... -count=1 -timeout 120s

race:
	bash scripts/race.sh

fallback:
	bash scripts/fallback.sh

timeout:
	bash scripts/timeout.sh

tidy:
	go mod tidy
