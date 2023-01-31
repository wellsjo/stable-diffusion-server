.PHONY: build
build:
	go build -o bin/ai-art .

.PHONY: start
start:
	docker compose up -d

.PHONY: down
down:
	docker compose down

.PHONY: clean
clean:
	docker volume rm aiart-pg-data

.PHONY: setup
setup:
	./setup.sh

.PHONY: test
test:
	go test -count=1 -v ./...

.PHONY: docker
docker:
	cd stable-diffusion-docker; docker build . --tag "ai-art"
