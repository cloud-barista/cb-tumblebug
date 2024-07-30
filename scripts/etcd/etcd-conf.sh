#!/bin/bash

ETCD_VERSION=${ETCD_VERSION_TAG:-v3.5.14}

ENDPOINTS=${ETCD_ENDPOINTS:-"http://etcd:2379"}
ETCD_BINS=${ETCD_PATH:-"/tmp/etcd-download-test"}
ETCD_CTL="${ETCD_BINS}/etcdctl --endpoints=${ENDPOINTS}"

AUTH_ENABLED=${ETCD_AUTH_ENABLED:-true}
ROOT_USERNAME="root" # Require 'root' to enable authentication
ROOT_PASSWORD=${ETCD_ROOT_PASSWORD:-default}
ADMIN_USERNAME=${ETCD_ADMIN_USERNAME:-default}
ADMIN_PASSWORD=${ETCD_ADMIN_PASSWORD:-default}


# Choose either URL
GOOGLE_URL=https://storage.googleapis.com/etcd
GITHUB_URL=https://github.com/etcd-io/etcd/releases/download
DOWNLOAD_URL=${GOOGLE_URL}


# Update the package list quietly, but show errors
apk update > /dev/null
# Install curl and tar quietly, but show errors
apk add --no-cache curl tar > /dev/null

# Clean up any previous downloads and create the target directory
rm -f /tmp/etcd-${ETCD_VERSION}-linux-amd64.tar.gz
rm -rf ${ETCD_BINS} && mkdir -p ${ETCD_BINS}

# Download the etcd tarball quietly, but show errors
curl -sSL ${DOWNLOAD_URL}/${ETCD_VERSION}/etcd-${ETCD_VERSION}-linux-amd64.tar.gz -o /tmp/etcd-${ETCD_VERSION}-linux-amd64.tar.gz
# Extract the tarball to the target directory quietly, but show errors
tar xzf /tmp/etcd-${ETCD_VERSION}-linux-amd64.tar.gz -C ${ETCD_BINS} --strip-components=1 2>&1
# Clean up the downloaded tarball
rm -f /tmp/etcd-${ETCD_VERSION}-linux-amd64.tar.gz

# Enable auth if AUTH_ENABLED is true
if [ "$AUTH_ENABLED" = "true" ]; then

    echo "AUTH_ENABLED is true"

    # Try to check auth status without authentication
    RET_CHECK_AUTH=$($ETCD_CTL auth status 2>/dev/null)
    if [ -z "$RET_CHECK_AUTH" ]; then
        # Check again auth status with authentication
        RET_CHECK_AUTH=$($ETCD_CTL --user ${ROOT_USERNAME}:${ROOT_PASSWORD} auth status)
    fi   

    if ! echo "$RET_CHECK_AUTH" | grep -q "Authentication Status: true"; then

        echo "Authentication is disabled. Enable it now."

        ROOT_ROLE="root"
        ADMIN_ROLE="admin"

        ## Set 'root'
        # Add 'root' role
        $ETCD_CTL role add ${ROOT_ROLE}

        # Add user 'root' with the password
        $ETCD_CTL user add ${ROOT_USERNAME}:${ROOT_PASSWORD}

        # Grant 'root' role to 'root' user
        $ETCD_CTL user grant-role ${ROOT_USERNAME} ${ROOT_ROLE}

        ## Set 'admin'
        # Add 'admin' user with the password
        $ETCD_CTL user add ${ADMIN_USERNAME}:${ADMIN_PASSWORD}

        # Add 'admin' role
        $ETCD_CTL role add ${ADMIN_ROLE}

        # Give full access permission to 'admin' role
        $ETCD_CTL role grant-permission ${ADMIN_ROLE} --prefix=true readwrite /


        # Grant 'admin' role to the 'admin' user
        $ETCD_CTL user grant-role ${ADMIN_USERNAME} ${ADMIN_ROLE}

        ## Enable authentication
        # Enable auth
        $ETCD_CTL auth enable

    else

        echo "Authentication is already enabled. Nothing to do."

    fi

# Disable auth if AUTH_ENABLED is false
elif [ "$AUTH_ENABLED" = "false" ]; then
    
    echo "AUTH_ENABLED is false"
    
    # Try to check auth status without authentication
    RET_CHECK_AUTH=$($ETCD_CTL auth status 2>/dev/null)
    if [ -z "$RET_CHECK_AUTH" ]; then
        # Check again auth status with authentication
        RET_CHECK_AUTH=$($ETCD_CTL --user ${ROOT_USERNAME}:${ROOT_PASSWORD} auth status)
    fi
    
    if ! echo "$RET_CHECK_AUTH" | grep -q "Authentication Status: false"; then

        echo "Authentication is enabled. Disable it now."

        ROOT_ROLE="root"
        ADMIN_ROLE="admin"

        ## Disable authentication
        # Disable auth
        $ETCD_CTL --user ${ROOT_USERNAME}:${ROOT_PASSWORD} auth disable

        ## Reset 'admin'
        # Revoke 'admin' role from 'admin' user
        $ETCD_CTL user revoke-role ${ADMIN_USERNAME} ${ADMIN_ROLE}

        # Delete 'admin' user
        $ETCD_CTL user delete ${ADMIN_USERNAME}

        # Delete 'admin' role
        $ETCD_CTL role delete ${ADMIN_ROLE}

        ## Reset 'root'
        # Revoke root role from root user
        $ETCD_CTL user revoke-role ${ROOT_USERNAME} ${ROOT_ROLE}

        # Delete 'root' user
        $ETCD_CTL user delete ${ROOT_USERNAME}

        # Delete 'root' role
        $ETCD_CTL role delete ${ROOT_ROLE}

    else 

        echo "Authentication is already disabled. Nothing to do."

    fi    

else
    echo "Invalid value for AUTH_ENABLED. Please set it to 'true' or 'false'."
    exit 1
fi

# Indicate successful completion
touch /tmp/healthcheck

# Add this line at the end of the script to handle termination signals
trap 'exit 0' SIGTERM SIGINT