#!/bin/bash

CONTAINER_NAME_READ="CB-Tumblebug"
CONTAINER_VERSION="latest"
CONTAINER_PORT="-p 1323:1323 -p 50252:50252"
CONTAINER_DATA_PATH="/app/meta_db"

parent_path=$( cd "$(dirname "${BASH_SOURCE[0]}")" ; pwd -P )
cd "$parent_path"

./runContainer.sh "$CONTAINER_NAME_READ" "$CONTAINER_VERSION" "$CONTAINER_PORT" "$CONTAINER_DATA_PATH"
