#!/bin/bash

SCRIPT_DIR=$(cd $(dirname "$0") && pwd)

# Python version check
PYTHON_VERSION=$(python3 --version | cut -d' ' -f2)
echo "Detected Python version: $PYTHON_VERSION"
PYTHON_MAJOR=$(echo $PYTHON_VERSION | cut -d. -f1)
PYTHON_MINOR=$(echo $PYTHON_VERSION | cut -d. -f2)

# Install python3-venv
if ! dpkg -s python${PYTHON_MAJOR}.${PYTHON_MINOR}-venv &> /dev/null; then
    echo "python3-venv package for Python ${PYTHON_MAJOR}.${PYTHON_MINOR} is not installed. Installing..."
    sudo apt update
    sudo apt install -y python${PYTHON_MAJOR}.${PYTHON_MINOR}-venv
    if [[ $? -ne 0 ]]; then
        echo "Failed to install python${PYTHON_MAJOR}.${PYTHON_MINOR}-venv. Please check the package availability or use DeadSnakes PPA."
        exit 1
    fi
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
