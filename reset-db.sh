#!/usr/bin/env bash

docker kill aiart-db 2>/dev/null
docker rm aiart-db 2>/dev/null
docker compose down

[ "$(docker volume ls -q -f name=aiart-pg-data)" ] && docker volume rm aiart-pg-data
docker volume create aiart-pg-data

docker compose up -d

echo "setting up postgres..."
timeout_counter=0
while true; do
  if [[ "$(docker logs --tail 100 ai-art-db 2>/dev/null)" = *"database system is ready to accept connections"* ]]; then
    break
  fi
  if [[ $timeout_counter == 10 ]]; then
    echo "could not connect to postgres"
    exit 1
  fi
  sleep 1
  echo -e ".\c"
  timeout_counter=$((timeout_counter + 1))
done

go run db/reset/reset.go

docker compose down
