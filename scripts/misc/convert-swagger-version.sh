#!/bin/bash

SWAGGER_FILE="$(dirname "$0")/../../src/interface/rest/docs/swagger.yaml"
SWAGGER_FILE_v3="$SWAGGER_FILE"

echo "Converting Swagger 2.0 to 3.0.1..."

if [ ! -f "$SWAGGER_FILE" ]; then
  echo "Error: $SWAGGER_FILE does not exist at $SWAGGER_FILE."
  exit 1
fi

# Converting tool: https://converter.swagger.io/#/Converter/convertByContent
curl -X 'POST' \
  'https://converter.swagger.io/api/convert' \
  -H 'accept: application/yaml' \
  -H 'Content-Type: application/yaml' \
  --data-binary @"$SWAGGER_FILE" \
  -o "$SWAGGER_FILE_v3"

if [ $? -eq 0 ]; then
  echo "Conversion complete. Updated $SWAGGER_FILE_v3"

  echo "Adding security section to the swagger.yaml file..."
  echo -e "\nsecurity:\n  - BasicAuth: []\n  - Bearer: []" >> "$SWAGGER_FILE_v3"
  echo "Security section added successfully."

else
  echo "Conversion failed."
fi