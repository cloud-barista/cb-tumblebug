#!/bin/bash

CONTAINER_NAME_READ="mc-terrarium"
CONTAINER_VERSION="0.0.6"
CONTAINER_PORT="-p 8055:8055"
CONTAINER_DATA_PATH="/app/.tofu"

if [ -z "$TB_ROOT_PATH" ]; then
    SCRIPT_DIR=$(dirname "${BASH_SOURCE[0]:-$0}")
    export TB_ROOT_PATH=$(cd "$SCRIPT_DIR" && cd .. && pwd)
fi

parent_path=$( cd "$(dirname "${BASH_SOURCE[0]}")" ; pwd -P )
cd "$parent_path"

# The credential directory and file path
credential_dir="$TB_ROOT_PATH/conf/.credtmp"
aws_credential="$credential_dir/credentials"
azure_credential="$credential_dir/credential-azure.env"
gcp_credential="$credential_dir/credential-gcp.json"

# The exported credentials from the credential.conf file
exported_aws_credential="$credential_dir/aws_credential"
exported_azure_credential="$credential_dir/azure_credential"
exported_gcp_credential="$credential_dir/gcp_credential.json"

# Check if credential files exist
if [ ! -f "$aws_credential" ] || [ ! -f "$gcp_credential" ] || [ ! -f "$azure_credential" ]; then

    # Check if the exported credentials exist
    if [ ! -f "$exported_aws_credential" ] || [ ! -f "$exported_gcp_credential" ] || [ ! -f "$exported_azure_credential" ]; then
        ./exportCredentials.sh
    fi    
    # Move the exported credentials to the credential directory
    mv "$exported_aws_credential" "$aws_credential"
    mv "$exported_gcp_credential" "$gcp_credential"
    mv "$exported_azure_credential" "$azure_credential"
fi

CONTAINER_ENV="--env-file $credential_dir/credentials \
--env-file $credential_dir/credential-azure.env \
--mount type=bind,source=$credential_dir/,target=/app/secrets/"

./runContainer.sh "$CONTAINER_NAME_READ" "$CONTAINER_VERSION" "$CONTAINER_PORT" "$CONTAINER_DATA_PATH" "$CONTAINER_ENV"
