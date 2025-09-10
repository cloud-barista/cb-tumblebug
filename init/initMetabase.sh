#!/bin/bash

# Copyright 2019 The Cloud-Barista Authors.
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#     http://www.apache.org/licenses/LICENSE-2.0
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# initMetabase.sh
# This script initializes Metabase with CB-Tumblebug PostgreSQL database connection
# It automatically sets up the admin user and connects to the CB-Tumblebug database for analytics

set -e

# Configuration
MB_HOST=${MB_HOST:-cb-tumblebug-metabase}
MB_PORT=${MB_PORT:-3000}
POSTGRES_HOST=${POSTGRES_HOST:-cb-tumblebug-postgres}
POSTGRES_PORT=${POSTGRES_PORT:-5432}
POSTGRES_DB=${POSTGRES_DB:-tumblebug}
POSTGRES_USER=${POSTGRES_USER:-tumblebug}
POSTGRES_PASSWORD=${POSTGRES_PASSWORD:-tumblebug}

# Admin user configuration
ADMIN_EMAIL=${ADMIN_EMAIL:-admin1@your.org}
ADMIN_PASSWORD=${ADMIN_PASSWORD:-admin1@your.org}
ADMIN_FIRST_NAME=${ADMIN_FIRST_NAME:-Cloud}
ADMIN_LAST_NAME=${ADMIN_LAST_NAME:-Barista}

# Site configuration
SITE_NAME=${SITE_NAME:-CB-Tumblebug Analytics}
SITE_LOCALE=${SITE_LOCALE:-en}

echo "=================================================="
echo "CB-Tumblebug Metabase Initialization Script"
echo "=================================================="
echo "Metabase Host: ${MB_HOST}:${MB_PORT}"
echo "PostgreSQL Host: ${POSTGRES_HOST}:${POSTGRES_PORT}"
echo "Target Database: ${POSTGRES_DB}"
echo "=================================================="

# Function to check if Metabase is ready
wait_for_metabase() {
    echo "Waiting for Metabase to be ready..."
    local max_attempts=60 
    local attempt=1
    
    while [ $attempt -le $max_attempts ]; do
        # Check if Metabase is responding via session properties
        local response
        local http_code
        
        response=$(curl -s -w "%{http_code}" "http://${MB_HOST}:${MB_PORT}/api/session/properties" 2>/dev/null || echo "000")
        http_code="${response: -3}"
        response="${response%???}"
        
        # Check if we get a valid response
        if [ "$http_code" = "200" ]; then
            echo "Metabase is ready! (attempt $attempt/$max_attempts)"
            return 0
        elif [ "$http_code" = "503" ]; then
            echo "Metabase is still initializing... (attempt $attempt/$max_attempts)"
        else
            echo "Metabase is not ready yet. HTTP $http_code (attempt $attempt/$max_attempts)"
        fi
        
        sleep 10
        attempt=$((attempt + 1))
    done
    
    echo "ERROR: Metabase did not become ready within the expected time"
    return 1
}

# Function to check if PostgreSQL is ready
wait_for_postgres() {
    echo "Checking PostgreSQL connection..."
    local max_attempts=10
    local attempt=1
    
    while [ $attempt -le $max_attempts ]; do
        if curl -s "http://${POSTGRES_HOST}:${POSTGRES_PORT}" > /dev/null 2>&1 || \
           nc -z "${POSTGRES_HOST}" "${POSTGRES_PORT}" 2>/dev/null; then
            echo "PostgreSQL is accessible! (attempt $attempt/$max_attempts)"
            return 0
        fi
        
        echo "Waiting for PostgreSQL... (attempt $attempt/$max_attempts)"
        sleep 5
        attempt=$((attempt + 1))
    done
    
    echo "WARNING: Could not verify PostgreSQL connection, but proceeding..."
    return 0
}

# Function to perform initial setup
perform_initial_setup() {
    echo "Getting setup token..."
    local setup_response
    local http_code
    
    # First check session properties for setup token
    setup_response=$(curl -s -w "%{http_code}" "http://${MB_HOST}:${MB_PORT}/api/session/properties" 2>/dev/null || echo "000")
    http_code="${setup_response: -3}"
    setup_response="${setup_response%???}"
    
    if [ "$http_code" != "200" ]; then
        echo "Cannot access session properties. HTTP $http_code"
        return 1
    fi
    
    # Check if setup is needed
    local has_user_setup
    has_user_setup=$(echo "$setup_response" | jq -r '."has-user-setup" // false' 2>/dev/null || echo "false")
    
    if [ "$has_user_setup" = "true" ]; then
        echo "Metabase already has users configured."
        return 1
    fi
    
    # Try to get setup token from /api/setup
    echo "Attempting to get setup token from /api/setup..."
    local token_response
    token_response=$(curl -s -w "%{http_code}" "http://${MB_HOST}:${MB_PORT}/api/setup" 2>/dev/null || echo "000")
    local token_http_code="${token_response: -3}"
    token_response="${token_response%???}"
    
    local setup_token=""
    if [ "$token_http_code" = "200" ]; then
        setup_token=$(echo "$token_response" | jq -r '.token // empty' 2>/dev/null || echo "")
    fi
    
    # If no token from /api/setup, try session properties
    if [ -z "$setup_token" ]; then
        echo "No token from /api/setup, trying session properties..."
        setup_token=$(echo "$setup_response" | jq -r '."setup-token" // empty' 2>/dev/null || echo "")
    fi
    
    if [ -n "$setup_token" ]; then
        echo "Found setup token: ${setup_token:0:10}..."
        echo "Performing initial setup with CB-Tumblebug database connection..."
        
        # Create the setup request
        local setup_data
        setup_data=$(cat <<EOF
{
  "token": "${setup_token}",
  "user": {
    "email": "${ADMIN_EMAIL}",
    "password": "${ADMIN_PASSWORD}",
    "first_name": "${ADMIN_FIRST_NAME}",
    "last_name": "${ADMIN_LAST_NAME}"
  },
  "database": {
    "engine": "postgres",
    "name": "CB-Tumblebug Database",
    "details": {
      "host": "${POSTGRES_HOST}",
      "port": ${POSTGRES_PORT},
      "dbname": "${POSTGRES_DB}",
      "user": "${POSTGRES_USER}",
      "password": "${POSTGRES_PASSWORD}",
      "ssl": false,
      "tunnel-enabled": false
    }
  },
  "prefs": {
    "site_name": "${SITE_NAME}",
    "site_locale": "${SITE_LOCALE}",
    "allow_tracking": false
  }
}
EOF
        )
        
        # Perform the setup
        local setup_result
        setup_result=$(curl -X POST \
            "http://${MB_HOST}:${MB_PORT}/api/setup" \
            -H 'Content-Type: application/json' \
            -d "${setup_data}" \
            -s -w "%{http_code}" -o /tmp/setup_response.json 2>/dev/null || echo "000")
        
        if [ "$setup_result" = "200" ] || [ "$setup_result" = "201" ]; then
            echo "✅ Initial setup completed successfully!"
            
            # Check if database was connected during setup
            local session_id
            if [ -f /tmp/setup_response.json ]; then
                session_id=$(jq -r '.id // empty' /tmp/setup_response.json 2>/dev/null || echo "")
            fi
            
            # Always try to add CB-Tumblebug database after setup
            if [ -n "$session_id" ]; then
                echo "🔍 Checking if CB-Tumblebug database needs to be added..."
                
                # Check existing databases
                local db_check_result
                db_check_result=$(curl -s \
                    "http://${MB_HOST}:${MB_PORT}/api/database" \
                    -H "X-Metabase-Session: ${session_id}" \
                    -w "%{http_code}" 2>/dev/null || echo "000")
                local db_http_code="${db_check_result: -3}"
                db_check_result="${db_check_result%???}"
                
                if [ "$db_http_code" = "200" ]; then
                    # Check if CB-Tumblebug database already exists
                    local has_tumblebug_db
                    has_tumblebug_db=$(echo "$db_check_result" | jq -r '.data[]? | select(.name == "CB-Tumblebug Database") | .id' 2>/dev/null || echo "")
                    
                    if [ -n "$has_tumblebug_db" ]; then
                        echo "✅ CB-Tumblebug database already connected during setup!"
                    else
                        echo "⚠️  CB-Tumblebug database not found, adding it manually..."
                        add_database_manually "$session_id"
                        if [ $? -eq 0 ]; then
                            echo "✅ CB-Tumblebug database added successfully!"
                        else
                            echo "⚠️  Failed to add CB-Tumblebug database manually"
                        fi
                    fi
                else
                    echo "⚠️  Could not verify database list. HTTP $db_http_code"
                fi
            else
                echo "⚠️  No session ID available to verify database connection"
            fi
            
            return 0
        else
            echo "⚠️  Setup request returned HTTP $setup_result"
            if [ -f /tmp/setup_response.json ]; then
                echo "Response: $(cat /tmp/setup_response.json)"
            fi
            return 1
        fi
    else
        echo "No setup token found. Metabase may already be configured."
        return 1
    fi
}

# Function to manually add CB-Tumblebug database
add_database_manually() {
    local session_id="$1"
    
    if [ -z "$session_id" ]; then
        echo "⚠️  No session ID provided for manual database addition"
        return 1
    fi
    
    echo "Adding CB-Tumblebug database manually..."
    
    # Create database connection data
    local db_data
    db_data=$(cat <<EOF
{
  "engine": "postgres",
  "name": "CB-Tumblebug Database",
  "details": {
    "host": "${POSTGRES_HOST}",
    "port": ${POSTGRES_PORT},
    "dbname": "${POSTGRES_DB}",
    "user": "${POSTGRES_USER}",
    "password": "${POSTGRES_PASSWORD}",
    "ssl": false,
    "tunnel-enabled": false
  },
  "auto_run_queries": true,
  "is_full_sync": true,
  "schedules": {}
}
EOF
    )
    
    # Add the database
    local add_db_result
    add_db_result=$(curl -X POST \
        "http://${MB_HOST}:${MB_PORT}/api/database" \
        -H 'Content-Type: application/json' \
        -H "X-Metabase-Session: ${session_id}" \
        -d "${db_data}" \
        -s -w "%{http_code}" -o /tmp/add_db_response.json 2>/dev/null || echo "000")
    
    if [ "$add_db_result" = "200" ] || [ "$add_db_result" = "201" ]; then
        echo "✅ CB-Tumblebug database added successfully!"
        
        # Get the database ID for sync
        local db_id
        if [ -f /tmp/add_db_response.json ]; then
            db_id=$(jq -r '.id // empty' /tmp/add_db_response.json 2>/dev/null || echo "")
            if [ -n "$db_id" ]; then
                echo "📡 Triggering database schema sync (ID: $db_id)..."
                
                # Trigger database sync
                curl -X POST \
                    "http://${MB_HOST}:${MB_PORT}/api/database/${db_id}/sync_schema" \
                    -H "X-Metabase-Session: ${session_id}" \
                    -s > /dev/null 2>&1
                
                echo "✅ Database schema sync initiated!"
            fi
        fi
        
        return 0
    else
        echo "⚠️  Failed to add database. HTTP $add_db_result"
        if [ -f /tmp/add_db_response.json ]; then
            echo "Response: $(cat /tmp/add_db_response.json)"
        fi
        return 1
    fi
}

# Function to add database connection for already configured Metabase
add_database_connection() {
    echo "Metabase is already configured. Checking database connection..."
    
    # Login to get session
    local login_data
    login_data=$(cat <<EOF
{
  "username": "${ADMIN_EMAIL}",
  "password": "${ADMIN_PASSWORD}"
}
EOF
    )
    
    local login_response
    login_response=$(curl -X POST \
        "http://${MB_HOST}:${MB_PORT}/api/session" \
        -H 'Content-Type: application/json' \
        -d "${login_data}" \
        -s -w "%{http_code}" -o /tmp/login_response.json 2>/dev/null || echo "000")
    
    local login_http_code="${login_response: -3}"
    local session_id=""
    
    if [ "$login_http_code" = "200" ] && [ -f /tmp/login_response.json ]; then
        session_id=$(jq -r '.id // empty' /tmp/login_response.json 2>/dev/null || echo "")
        
        if [ -n "$session_id" ]; then
            echo "✅ Successfully logged in to Metabase"
            
            # Check existing databases
            local db_list_response
            db_list_response=$(curl -s \
                "http://${MB_HOST}:${MB_PORT}/api/database" \
                -H "X-Metabase-Session: ${session_id}" \
                -w "%{http_code}" 2>/dev/null || echo "000")
            
            local db_list_http_code="${db_list_response: -3}"
            db_list_response="${db_list_response%???}"
            
            if [ "$db_list_http_code" = "200" ]; then
                # Check if CB-Tumblebug database already exists
                local has_tumblebug_db
                has_tumblebug_db=$(echo "$db_list_response" | jq -r '.data[]? | select(.name == "CB-Tumblebug Database") | .id' 2>/dev/null || echo "")
                
                if [ -n "$has_tumblebug_db" ]; then
                    echo "✅ CB-Tumblebug database already exists (ID: $has_tumblebug_db)"
                    return 0
                else
                    echo "⚠️  CB-Tumblebug database not found, adding it..."
                    add_database_manually "$session_id"
                    return $?
                fi
            else
                echo "⚠️  Failed to get database list. HTTP $db_list_http_code"
                return 1
            fi
        else
            echo "⚠️  Failed to extract session ID from login response"
            return 1
        fi
    else
        echo "⚠️  Login failed. HTTP $login_http_code"
        if [ -f /tmp/login_response.json ]; then
            echo "Response: $(cat /tmp/login_response.json)"
        fi
        return 1
    fi
    
    local session_id
    session_id=$(echo "$login_response" | grep -o '"id":"[^"]*' | cut -d'"' -f4 2>/dev/null || echo "")
    
    if [ -n "$session_id" ]; then
        echo "Successfully logged in. Session: ${session_id:0:10}..."
        
        # Get existing databases
        local databases_response
        databases_response=$(curl -X GET \
            "http://${MB_HOST}:${MB_PORT}/api/database" \
            -H "X-Metabase-Session: ${session_id}" \
            -s 2>/dev/null || echo '{}')
        
        # Check if CB-Tumblebug database already exists
        if echo "$databases_response" | grep -q "CB-Tumblebug Database"; then
            echo "✅ CB-Tumblebug Database connection already exists!"
            return 0
        fi
        
        echo "Adding CB-Tumblebug Database connection..."
        
        local db_data
        db_data=$(cat <<EOF
{
  "engine": "postgres",
  "name": "CB-Tumblebug Database",
  "details": {
    "host": "${POSTGRES_HOST}",
    "port": ${POSTGRES_PORT},
    "dbname": "${POSTGRES_DB}",
    "user": "${POSTGRES_USER}",
    "password": "${POSTGRES_PASSWORD}",
    "ssl": false,
    "tunnel-enabled": false
  },
  "is_full_sync": true,
  "schedules": {
    "metadata_sync": {
      "schedule_type": "daily",
      "schedule_hour": 2
    },
    "cache_field_values": {
      "schedule_type": "daily",
      "schedule_hour": 3
    }
  }
}
EOF
        )
        
        local add_db_result
        add_db_result=$(curl -X POST \
            "http://${MB_HOST}:${MB_PORT}/api/database" \
            -H "Content-Type: application/json" \
            -H "X-Metabase-Session: ${session_id}" \
            -d "${db_data}" \
            -s -w "%{http_code}" -o /tmp/add_db_response.json 2>/dev/null || echo "000")
        
        if [ "$add_db_result" = "200" ] || [ "$add_db_result" = "201" ]; then
            echo "✅ CB-Tumblebug Database connection added successfully!"
            return 0
        else
            echo "⚠️  Failed to add database connection. HTTP $add_db_result"
            if [ -f /tmp/add_db_response.json ]; then
                echo "Response: $(cat /tmp/add_db_response.json)"
            fi
            return 1
        fi
    else
        echo "❌ Failed to login to Metabase. Please check credentials."
        return 1
    fi
}

# Main execution
main() {
    echo "Starting Metabase initialization..."
    
    # Wait for services to be ready
    wait_for_metabase || {
        echo "❌ Failed to connect to Metabase"
        exit 1
    }
    
    wait_for_postgres
    
    # Try initial setup first
    if perform_initial_setup; then
        echo "✅ Metabase initialization completed via initial setup!"
    else
        echo "Initial setup not available. Trying to add database connection to existing setup..."
        if add_database_connection; then
            echo "✅ Database connection verified/added successfully!"
        else
            echo "⚠️  Could not add database connection automatically."
            echo "Please manually add the CB-Tumblebug database in Metabase admin panel:"
            echo "  - Engine: PostgreSQL"
            echo "  - Host: ${POSTGRES_HOST}"
            echo "  - Port: ${POSTGRES_PORT}"
            echo "  - Database: ${POSTGRES_DB}"
            echo "  - Username: ${POSTGRES_USER}"
            echo "  - Password: ${POSTGRES_PASSWORD}"
        fi
    fi
    
    echo "=================================================="
    echo "Metabase Setup Information:"
    echo "=================================================="
    echo "📊 Metabase URL: http://localhost:3000"
    echo "👤 Admin Email: ${ADMIN_EMAIL}"
    echo "🔑 Admin Password: ${ADMIN_PASSWORD}"
    echo "🗄️  Database: ${POSTGRES_DB} on ${POSTGRES_HOST}:${POSTGRES_PORT}"
    echo "=================================================="
    echo "Metabase initialization script completed!"
}

# Cleanup function
cleanup() {
    echo "Cleaning up temporary files..."
    rm -f /tmp/setup_response.json /tmp/add_db_response.json
}

# Set trap for cleanup
trap cleanup EXIT

# Run main function
main "$@"
