#!/bin/bash

SCRIPT_DIR=$(cd $(dirname "$0") && pwd)

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
