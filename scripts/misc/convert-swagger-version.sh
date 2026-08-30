#!/bin/bash

DOCS_DIR="$(dirname "$0")/../../src/interface/rest/docs"
SWAGGER_YAML="$DOCS_DIR/swagger.yaml"

echo "Converting Swagger 2.0 to OpenAPI 3.0.1..."

# Check if YAML file exists
if [ ! -f "$SWAGGER_YAML" ]; then
  echo "Error: $SWAGGER_YAML does not exist."
  exit 1
fi

# Convert via remote converter into a temp file, validate, then replace.
# Writing the response directly over the source corrupts it on truncated/failed responses.
convert() {
  local src="$1" tmp
  tmp=$(mktemp)
  if ! curl --fail --silent --show-error --retry 2 -X 'POST' \
    'https://converter.swagger.io/api/convert' \
    -H "accept: application/yaml" \
    -H "Content-Type: application/yaml" \
    --data-binary @"$src" \
    -o "$tmp"; then
    rm -f "$tmp"
    return 1
  fi
  # Validate the converted document before replacing the original
  grep -q "^openapi:" "$tmp" || { rm -f "$tmp"; return 1; }
  mv "$tmp" "$src"
}

echo "Converting swagger.yaml..."
if ! convert "$SWAGGER_YAML"; then
  echo "Error: YAML conversion failed. Original file kept."
  exit 1
fi
echo "YAML conversion complete. Updated $SWAGGER_YAML"

echo "Swagger conversion completed successfully."
