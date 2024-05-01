#!/bin/bash

SCRIPT_DIR=$(cd $(dirname "$0") && pwd)

# Check if python3-venv is installed
if ! dpkg -s python3-venv &> /dev/null; then
    echo "python3-venv package is not installed. Installing..."
    sudo apt-get update && sudo apt-get install python3-venv
    if [ $? -ne 0 ]; then
        echo "Failed to install python3-venv. Please install it manually."
    fi
else
    echo "python3-venv package is already installed."
fi

echo "Creating and activating the virtual environment..."
python3 -m venv "$SCRIPT_DIR/initPyEnv"
source "$SCRIPT_DIR/initPyEnv/bin/activate"

echo
echo "Installing dependencies..."
pip3 install -r "$SCRIPT_DIR/requirements.txt"

echo
echo "Running the application..."
echo
python3 "$SCRIPT_DIR/init.py" "$@"

echo
echo "Cleaning up..."
deactivate

rm -rf "$SCRIPT_DIR/initPyEnv"
echo "Environment cleanup complete."
