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

# Convert YAML file
echo "Converting swagger.yaml..."
curl -X 'POST' \
  'https://converter.swagger.io/api/convert' \
  -H 'accept: application/yaml' \
  -H 'Content-Type: application/yaml' \
  --data-binary @"$SWAGGER_YAML" \
  -o "$SWAGGER_YAML"

if [ $? -ne 0 ]; then
  echo "Error: YAML conversion failed."
  exit 1
fi

echo "YAML conversion complete. Updated $SWAGGER_YAML"

# Convert JSON file
if [ -f "$SWAGGER_JSON" ]; then
  echo "Converting swagger.json..."
  curl -X 'POST' \
    'https://converter.swagger.io/api/convert' \
    -H 'accept: application/json' \
    -H 'Content-Type: application/json' \
    --data-binary @"$SWAGGER_JSON" \
    -o "$SWAGGER_JSON"
  
  if [ $? -eq 0 ]; then
    echo "JSON conversion complete. Updated $SWAGGER_JSON"
  else
    echo "Warning: JSON conversion failed, but continuing..."
  fi
else
  echo "Warning: swagger.json not found, skipping JSON conversion."
fi

echo "Swagger conversion completed successfully."