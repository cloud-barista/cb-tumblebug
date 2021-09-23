#!/bin/bash

CONTAINER_NAME_READ="CB-MapUI"
CONTAINER_VERSION="latest"
CONTAINER_PORT="-p 1324:1324"
CONTAINER_DATA_PATH="/app/dist"

parent_path=$( cd "$(dirname "${BASH_SOURCE[0]}")" ; pwd -P )
cd "$parent_path"

./runContainer.sh "$CONTAINER_NAME_READ" "$CONTAINER_VERSION" "$CONTAINER_PORT" "$CONTAINER_DATA_PATH"
