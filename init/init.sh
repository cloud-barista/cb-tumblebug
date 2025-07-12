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

# Ensure uv is installed
echo
echo "Checking for uv..."
if ! command -v uv &> /dev/null; then
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "uv is not installed"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "uv is an extremely fast Python package installer and resolver,"
    echo "designed as a drop-in replacement for pip and pip-tools."
    echo "It's required for this project to manage Python dependencies efficiently."
    echo
    echo "You can install it using one of these methods:"
    echo
    echo "Option 1: Direct install (recommended)"
    echo -e "\033[4;94mcurl -LsSf https://astral.sh/uv/install.sh | sh\033[0m"
    echo
    echo "Option 2: Visit the installation page"
    echo -e "\033[4;94mhttps://github.com/astral-sh/uv#installation\033[0m"
    echo
    echo "After installation, reload your shell environment with:"
    echo -e "\033[4;94msource ~/.bashrc\033[0m"
    echo "# or use source ~/.bash_profile or source ~/.profile depending on your system"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    exit 1
fi

echo
echo "Running the application..."
uv run init.py "$@"

echo
echo "Cleaning up the venv and uv.lock files..."
rm -rf .venv
rm -rf uv.lock # Make it commented out if you want to keep the lock file

echo
echo "Environment cleanup complete."

# Return to the original directory
popd > /dev/null
