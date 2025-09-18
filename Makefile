run:
	@echo ">> Running (dotenv)..."
	@set -a; . ./.env; set +a; go run ./cmd/api

build:
	@echo ">> Building..."
	@CGO_CFLAGS="-I/opt/homebrew/include" \
	CGO_LDFLAGS="-L/opt/homebrew/lib" \
	go build -o bin/lemme_api ./cmd/api

migrate:
	@echo ">> Running migrations..."
	@set -a; . ./.env; set +a; go run ./cmd/api --migrate

lint:
	@gofmt -s -w .

