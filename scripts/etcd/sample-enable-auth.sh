#!/bin/bash

ETCD_CONTAINER_NAME="etcd"
ETCD_CTL="docker exec ${ETCD_CONTAINER_NAME} etcdctl"

ROOT_USERNAME="root"
ROOT_PASSWORD="default"
ADMIN_USERNAME="default"
ADMIN_PASSWORD="default"

# Check if the etcd container is running
check_etcd() {
    if ! docker ps | grep -q ${ETCD_CONTAINER_NAME}; then
        echo "The etcd container is not running."
        exit 1
    fi
    
    while ! docker exec ${ETCD_CONTAINER_NAME} etcdctl endpoint health --cluster; do
        echo "The etcd service is not ready yet. Waiting..."
        sleep 5
    done
    echo "The etcd service is running normally."
}

# Check if authentication is already enabled
check_auth_enabled() {
    local RET_CHECK_AUTH
    RET_CHECK_AUTH=$($ETCD_CTL auth status)
    
    if echo "$RET_CHECK_AUTH" | grep -q "Authentication Status: true"; then
        echo "Authentication is already enabled. Exit the script"
        exit 1
    fi
}

# Set root password and enable authentication
set_root_user() {

    local RET_ADD_ROLE
    local RET_ADD_USER
    local RET_GRANT_ROLE
    local ROOT_ROLE="root"
    local ROOT_USERNAME="root" # Require 'root' to enable authentication

    # Get root password from the user
    while true; do
        echo -e "Enter root user password (press Enter for default password 'default'): \c"
        read -sp "" PASSWORD
        echo

        if [ -z "$PASSWORD" ]; then
            echo "Using default password."
            ROOT_PASSWORD="default"                        
            break
        fi

        if [ -n "$PASSWORD" ]; then
            read -sp "Confirm the password: " PASSWORD_CONFIRM
            echo

            if [ "$PASSWORD" = "$PASSWORD_CONFIRM" ]; then
                ROOT_PASSWORD=$PASSWORD
                break
            fi           
        fi
        echo "Passwords do not match. Please try again."
    done

    # Add 'root' role
    RET_ADD_ROLE=$($ETCD_CTL role add ${ROOT_ROLE})
    if ! echo "$RET_ADD_ROLE" | grep -q "Role ${ROOT_ROLE} created"; then
        echo $RET_ADD_ROLE
        echo "Exit the script"
        exit 1
    fi
    echo "Successfully added role (root)"

    # Add user 'root' with the password
    RET_ADD_USER=$($ETCD_CTL user add ${ROOT_USERNAME}:${ROOT_PASSWORD})
    if ! echo "$RET_ADD_USER" | grep -q "User ${ROOT_USERNAME} created"; then
        echo $RET_ADD_USER
        echo "Exit the script"
        exit 1
    fi

    echo "Successfully added, the user (root) with the password"

    # Grant 'root' role to 'root' user
    RET_GRANT_ROLE=$($ETCD_CTL user grant-role ${ROOT_USERNAME} ${ROOT_ROLE})
    if ! echo "$RET_GRANT_ROLE" | grep -q "Role ${ROOT_ROLE} is granted to user ${ROOT_USERNAME}"; then
        echo $RET_GRANT_ROLE
        echo "Exit the script"
        exit 1
    fi

    echo "Successfully granted, the role to user"
}

# Add additional admin user
add_additional_admin_user() {

    local RET_ADD_ROLE
    local RET_GRANT_PERMISSION
    local RET_ADD_USER
    local RET_GRANT_ROLE
    local ADMIN_ROLE="admin"

    # Get admin username from the user
    while true; do
        echo -e "Enter admin username (press Enter for default username 'default'): \c"
        read -sp "" USERNAME
        echo 

        if [ -z "$USERNAME" ]; then
            ADMIN_USERNAME="default"
            echo "Using default username 'default'."
            break
        elif [[ "$USERNAME" =~ ^[a-zA-Z0-9_]+$ ]]; then
            ADMIN_USERNAME=$USERNAME
            break
        else
            echo "Invalid admin username. Use only letters, numbers, and underscores. Please try again."
        fi
    done

    # Get admin password from the user
    while true; do
        echo -e "Enter user password (press Enter for default password 'default'): \c"
        read -sp "" PASSWORD
        echo

        if [ -z "$PASSWORD" ]; then
            ADMIN_PASSWORD="default"
            echo "Using default password."
            break
        fi

        if [ -n "$PASSWORD" ]; then
            read -sp "Confirm the password: " PASSWORD_CONFIRM
            echo

            if [ "$PASSWORD" = "$PASSWORD_CONFIRM" ]; then
                ADMIN_PASSWORD=$PASSWORD
                break
            fi           
        fi
        echo "Passwords do not match. Please try again."
    done

    # Add 'admin' role
    RET_ADD_ROLE=$($ETCD_CTL role add ${ADMIN_ROLE})
    if ! echo "$RET_ADD_ROLE" | grep -q "Role ${ADMIN_ROLE} created"; then
        echo $RET_ADD_ROLE
        echo "Exit the script"
        exit 1
    fi
    echo "Successfully added the role"

    # Give full access permission to 'admin' role
    RET_GRANT_PERMISSION=$($ETCD_CTL role grant-permission ${ADMIN_ROLE} --prefix=true readwrite / )
    if ! echo "$RET_GRANT_PERMISSION" | grep -q "Role ${ADMIN_ROLE} updated"; then
        echo $RET_GRANT_PERMISSION
        echo "Exit the script"
        exit 1
    fi

    # Add a user with the password
    RET_ADD_USER=$($ETCD_CTL user add ${ADMIN_USERNAME}:${ADMIN_PASSWORD})
    if ! echo "$RET_ADD_USER" | grep -q "User ${ADMIN_USERNAME} created"; then
        echo $RET_ADD_USER
        echo "Exit the script"
        exit 1
    fi

    echo "Successfully added, the user with the password"

    # Grant 'admin' role to the user
    RET_GRANT_ROLE=$($ETCD_CTL user grant-role ${ADMIN_USERNAME} ${ADMIN_ROLE})
    if ! echo "$RET_GRANT_ROLE" | grep -q "Role ${ADMIN_ROLE} is granted to user ${ADMIN_USERNAME}"; then
        echo $RET_GRANT_ROLE
        echo "Exit the script"
        exit 1
    fi

    echo "Successfully added, the additional admin user with full access permission"
}

enable_auth() {

    local RET_AUTH_ENABLE

    RET_AUTH_ENABLE=$($ETCD_CTL auth enable)
    if ! echo "$RET_AUTH_ENABLE" | grep -q "Authentication Enabled"; then
        echo "Failed to enable authentication."
        exit 1
    fi

    echo "Successfully enabled, the authentication"
}

# Main execution flow
main() {
    check_etcd
    check_auth_enabled
    set_root_user
    add_additional_admin_user
    enable_auth
    echo "Successfully completed authentication setup"
}

main
