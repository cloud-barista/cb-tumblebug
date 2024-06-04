#!/bin/bash

# Define the file path
FILE="/etc/nginx/sites-enabled/default"

# Check if the file exists
if [ ! -f "$FILE" ]; then
    echo "Error: $FILE does not exist."
    exit 1
fi

# Use sed to comment out the specified line if it is not already commented
sudo sed -i '/listen \[::\]:80 default_server;/ s/^/#/' "$FILE"

echo "Line commented successfully."
