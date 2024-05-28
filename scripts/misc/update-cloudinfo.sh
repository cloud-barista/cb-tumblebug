#!/bin/bash

SCRIPT_DIR=$(cd $(dirname "$0") && pwd)

# Python version check
REQUIRED_VERSION="3.8.0"

PYTHON_VERSION=$(python3 --version | cut -d' ' -f2)
echo "Detected Python version: $PYTHON_VERSION"
PYTHON_MAJOR=$(echo $PYTHON_VERSION | cut -d. -f1)
PYTHON_MINOR=$(echo $PYTHON_VERSION | cut -d. -f2)
PYTHON_PATCH=$(echo $PYTHON_VERSION | cut -d. -f3)

# Check if the Python3 version is 3.8.0 or higher
REQUIRED_MAJOR=3
REQUIRED_MINOR=8
REQUIRED_PATCH=0

if [[ $PYTHON_MAJOR -gt $REQUIRED_MAJOR ]] || \
   [[ $PYTHON_MAJOR -eq $REQUIRED_MAJOR && $PYTHON_MINOR -gt $REQUIRED_MINOR ]] || \
   [[ $PYTHON_MAJOR -eq $REQUIRED_MAJOR && $PYTHON_MINOR -eq $REQUIRED_MINOR && $PYTHON_PATCH -ge $REQUIRED_PATCH ]]; then
    echo "Python version is sufficient."
else
    echo "This script requires Python $REQUIRED_MAJOR.$REQUIRED_MINOR.$REQUIRED_PATCH or higher. Please upgrade the version"
    exit 1
fi

# Try creating the virtual environment first
echo "Attempting to create a virtual environment..."
if python3 -m venv "$SCRIPT_DIR/tmpPyEnv"; then
    echo "Virtual environment created successfully."
else
    echo "Failed to create the virtual environment. Checking for ensurepip..."
    # Check if venv module and ensurepip are available
    if ! python3 -c "import venv, ensurepip" &> /dev/null; then
        echo "venv or ensurepip module is not available. Installing python3-venv..."
        sudo apt update
        sudo apt install -y python${PYTHON_MAJOR}.${PYTHON_MINOR}-venv
        if [[ $? -ne 0 ]]; then
            echo "Failed to install python${PYTHON_MAJOR}.${PYTHON_MINOR}-venv. Please check the package availability."
            exit 1
        fi
        # Retry creating the virtual environment
        if ! python3 -m venv "$SCRIPT_DIR/tmpPyEnv"; then
            echo "Failed to create the virtual environment after installing python3-venv."
            exit 1
        fi
    fi
fi

# Activate the virtual environment
source "$SCRIPT_DIR/tmpPyEnv/bin/activate"

echo
echo "Installing dependencies..."
python3 -m pip install --upgrade pip
pip3 install -r "$SCRIPT_DIR/requirements.txt"

echo
echo "Running the application..."
echo
python3 "$SCRIPT_DIR/update-cloudinfo.py" "$@"

echo
echo "Cleaning up..."
deactivate

rm -rf "$SCRIPT_DIR/tmpPyEnv"
echo "Environment cleanup complete."
