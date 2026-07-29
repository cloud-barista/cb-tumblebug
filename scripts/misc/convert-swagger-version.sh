#!/bin/bash

DOCS_DIR="$(dirname "$0")/../../src/interface/rest/docs"
SWAGGER_YAML="$DOCS_DIR/swagger.yaml"
SWAGGER_JSON="$DOCS_DIR/swagger.json"

echo "Converting Swagger 2.0 to OpenAPI 3.0.1..."

# Check if YAML file exists
if [ ! -f "$SWAGGER_YAML" ]; then
  echo "Error: $SWAGGER_YAML does not exist."
  exit 1
fi

# Convert via remote converter into a temp file, validate, then replace.
# Writing the response directly over the source corrupts it on truncated/failed responses.
convert() {
  local src="$1" fmt="$2" tmp
  tmp=$(mktemp)
  if ! curl --fail --silent --show-error --retry 2 -X 'POST' \
    'https://converter.swagger.io/api/convert' \
    -H "accept: application/$fmt" \
    -H "Content-Type: application/$fmt" \
    --data-binary @"$src" \
    -o "$tmp"; then
    rm -f "$tmp"
    return 1
  fi
  # Validate the converted document before replacing the original
  if [ "$fmt" = "json" ]; then
    python3 -c "import json,sys; json.load(open(sys.argv[1]))" "$tmp" 2>/dev/null || { rm -f "$tmp"; return 1; }
  else
    grep -q "^openapi:" "$tmp" || { rm -f "$tmp"; return 1; }
  fi
  mv "$tmp" "$src"
}

# Convert YAML file
echo "Converting swagger.yaml..."
if ! convert "$SWAGGER_YAML" "yaml"; then
  echo "Error: YAML conversion failed. Original file kept."
  exit 1
fi
echo "YAML conversion complete. Updated $SWAGGER_YAML"

# Convert JSON file
if [ -f "$SWAGGER_JSON" ]; then
  echo "Converting swagger.json..."
  if convert "$SWAGGER_JSON" "json"; then
    echo "JSON conversion complete. Updated $SWAGGER_JSON"
  else
    echo "Warning: JSON conversion failed. Original file kept."
  fi
else
  echo "Warning: swagger.json not found, skipping JSON conversion."
fi

echo "Swagger conversion completed successfully."
