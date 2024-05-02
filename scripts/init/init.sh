#!/bin/bash

SCRIPT_DIR=$(cd $(dirname "$0") && pwd)

# Python version check
MIN_MAJOR=3
MIN_MINOR=8
if ! python3 -c "import sys; assert sys.version_info >= ($MIN_MAJOR, $MIN_MINOR), f'Python $MIN_MAJOR.$MIN_MINOR or newer is required'"; then
    echo "Your Python version is too old. Please upgrade to $MIN_MAJOR.$MIN_MINOR or newer."
    exit 1
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
