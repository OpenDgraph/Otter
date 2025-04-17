#!/usr/bin/env bash

ENDPOINT="${1:-http://localhost:8084/admin/schema}"
SCHEMA_FILE="${2:-schema.graphql}"

if [ ! -f "$SCHEMA_FILE" ]; then
  echo "Erro: Not found: $SCHEMA_FILE" >&2
  exit 1
fi

curl -s -X POST "$ENDPOINT" \
     -H "Content-Type: application/graphql" \
     --data-binary "@${SCHEMA_FILE}" \
  && echo -e "\n Schema sent $ENDPOINT"
