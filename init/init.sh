#!/bin/bash

SCRIPT_DIR=$(cd $(dirname "$0") && pwd)

# Change to the script directory
pushd "$SCRIPT_DIR" > /dev/null

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
    echo "  * Upgrade command by uv: uv python install $REQUIRED_MAJOR.$REQUIRED_MINOR"
    exit 1
fi

# Ensure curl is installed
echo
echo "Checking for curl..."
if ! command -v curl &> /dev/null; then
    echo "curl is not installed. Installing..."
    sudo apt update
    sudo apt install -y curl || {
        echo "Failed to install curl."
        exit 1
    }

    echo "curl is installed successfully."
else
    echo "curl is already installed."
fi


# Ensure uv is installed
echo
echo "Checking for uv..."
if ! command -v uv &> /dev/null; then
    echo "uv is not installed. Installing..."
    curl -LsSf https://astral.sh/uv/install.sh | sh
    if [[ $? -ne 0 ]]; then
        echo "Failed to install uv. Please check the installation script."
        exit 1
    fi

    source ~/.bashrc
    if ! command -v uv &> /dev/null; then
        echo "uv installation failed. Please check your PATH."
        exit 1
    fi

    echo "uv is installed successfully."
else
    echo "uv is already installed."
fi

echo
echo "Running the application..."
uv run init.py "$@"

echo
echo "Cleaning up..."
rm -rf .venv
rm -rf uv.lock # Make it commented out if you want to keep the lock file

echo
echo "Environment cleanup complete."

# Return to the original directory
popd > /dev/null
