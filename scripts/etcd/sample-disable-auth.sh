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

get_root_username_and_password() {
    # Get root username from the user
    while true; do
        echo -e "Enter root username (press Enter for default username 'root'): \c"
        read -sp "" USERNAME
        echo 

        if [ -z "$USERNAME" ]; then
            ROOT_USERNAME="root"
            echo "Using default username 'default'."
            break
        elif [[ "$USERNAME" =~ ^[a-zA-Z0-9_]+$ ]]; then
            ROOT_USERNAME=$USERNAME
            break
        else
            echo "Invalid root username. Use only letters, numbers, and underscores. Please try again."
        fi
    done

    # Get root password from the user
    while true; do
        echo -e "Enter root password (press Enter for default password 'default'): \c"
        read -sp "" PASSWORD
        echo

        if [ -z "$PASSWORD" ]; then
            ROOT_PASSWORD="default"
            echo "Using default password."
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
}

get_admin_username_and_password() {
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
        echo -e "Enter admin password (press Enter for default password 'default'): \c"
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
}

# Check if authentication is already disabled
check_auth_disabled() {
    local RET_CHECK_AUTH
    RET_CHECK_AUTH=$($ETCD_CTL --user ${ROOT_USERNAME}:${ROOT_PASSWORD} auth status)
    
    if echo "$RET_CHECK_AUTH" | grep -q "Authentication Status: false"; then
        echo "Authentication is already disabled. Exit the script"
        exit 1
    fi
}

# Disable authentication
disable_auth() {
    local RET_AUTH_DISABLE

    RET_AUTH_DISABLE=$($ETCD_CTL --user ${ROOT_USERNAME}:${ROOT_PASSWORD} auth disable)
    if ! echo "$RET_AUTH_DISABLE" | grep -q "Authentication Disabled"; then
        echo "Failed to disable authentication."
        exit 1
    fi

    echo "Successfully disabled the authentication"
}

# Remove the root and admin users and their roles
remove_users_and_roles() {
    local ROOT_USERNAME="root"
    local ROOT_ROLE="root"
    local ADMIN_ROLE="admin"
    local RET_REVOKE_ROLE
    local RET_DELETE_USER
    local RET_DELETE_ROLE

    # Revoke 'admin' role from 'admin' user
    RET_REVOKE_ROLE=$($ETCD_CTL user revoke-role ${ADMIN_USERNAME} ${ADMIN_ROLE})
    if ! echo "$RET_REVOKE_ROLE" | grep -q "Role ${ADMIN_ROLE} is revoked from user ${ADMIN_USERNAME}"; then
        echo $RET_REVOKE_ROLE
    fi

    # Delete 'admin' user
    RET_DELETE_USER=$($ETCD_CTL user delete ${ADMIN_USERNAME})
    if ! echo "$RET_DELETE_USER" | grep -q "User ${ADMIN_USERNAME} deleted"; then
        echo $RET_DELETE_USER
    fi

    # Delete 'admin' role
    RET_DELETE_ROLE=$($ETCD_CTL role delete ${ADMIN_ROLE})
    if ! echo "$RET_DELETE_ROLE" | grep -q "Role ${ADMIN_ROLE} deleted"; then
        echo $RET_DELETE_ROLE
    fi

    # Revoke 'root' role from 'root' user
    RET_REVOKE_ROLE=$($ETCD_CTL user revoke-role ${ROOT_USERNAME} ${ROOT_ROLE})
    if ! echo "$RET_REVOKE_ROLE" | grep -q "Role ${ROOT_ROLE} is revoked from user ${ROOT_USERNAME}"; then
        echo $RET_REVOKE_ROLE
    fi

    # Delete 'root' user
    RET_DELETE_USER=$($ETCD_CTL user delete ${ROOT_USERNAME})
    if ! echo "$RET_DELETE_USER" | grep -q "User ${ROOT_USERNAME} deleted"; then
        echo $RET_DELETE_USER
    fi

    # Delete 'root' role
    RET_DELETE_ROLE=$($ETCD_CTL role delete ${ROOT_ROLE})
    if ! echo "$RET_DELETE_ROLE" | grep -q "Role ${ROOT_ROLE} deleted"; then
        echo $RET_DELETE_ROLE
    fi

    echo "Successfully removed the root and admin users and their roles"
}

# Main execution flow
main() {
    check_etcd
    check_auth_disabled
    get_root_username_and_password
    get_admin_username_and_password
    disable_auth
    remove_users_and_roles
    echo "Successfully disabled authentication and removed users/roles"
}

main