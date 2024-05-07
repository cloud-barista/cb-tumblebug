#!/bin/bash

SCRIPT_DIR=$(cd $(dirname "$0") && pwd)

# Python version check
REQUIRED_VERSION="3.7.5"

PYTHON_VERSION=$(python3 --version | cut -d' ' -f2)
echo "Detected Python version: $PYTHON_VERSION"
PYTHON_MAJOR=$(echo $PYTHON_VERSION | cut -d. -f1)
PYTHON_MINOR=$(echo $PYTHON_VERSION | cut -d. -f2)
PYTHON_PATCH=$(echo $PYTHON_VERSION | cut -d. -f3)

# Check if the Python3 version is 3.7.5 or higher
REQUIRED_MAJOR=3
REQUIRED_MINOR=7
REQUIRED_PATCH=5

if [[ $PYTHON_MAJOR -gt $REQUIRED_MAJOR ]] || \
   [[ $PYTHON_MAJOR -eq $REQUIRED_MAJOR && $PYTHON_MINOR -gt $REQUIRED_MINOR ]] || \
   [[ $PYTHON_MAJOR -eq $REQUIRED_MAJOR && $PYTHON_MINOR -eq $REQUIRED_MINOR && $PYTHON_PATCH -ge $REQUIRED_PATCH ]]; then
    echo "Python version is sufficient."
else
    echo "This script requires Python $REQUIRED_MAJOR.$REQUIRED_MINOR.$REQUIRED_PATCH or higher. Please upgrade the version"
    exit 1
fi

# Check if venv module is available and python3-venv is installed
if ! python3 -c "import venv ensurepip" &> /dev/null; then
    if ! dpkg -s python${PYTHON_MAJOR}.${PYTHON_MINOR}-venv &> /dev/null; then
        echo "python3-venv package for Python ${PYTHON_MAJOR}.${PYTHON_MINOR} is not installed. Installing..."
        sudo apt update
        sudo apt install -y python${PYTHON_MAJOR}.${PYTHON_MINOR}-venv
        if [[ $? -ne 0 ]]; then
            echo "Failed to install python${PYTHON_MAJOR}.${PYTHON_MINOR}-venv. Please check the package availability or use DeadSnakes PPA."
            exit 1
        fi
    fi
else
    echo "venv module is available."
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
