
# README.md for `init.py`

## Overview
The `init.py` script is designed to automate the process of registering credentials and loading common specifications and images for a Tumblebug server. It can be executed directly or via the `init.sh` script which sets up a Python virtual environment. This script ensures the Tumblebug server is healthy before proceeding and performs several network operations in a secure and managed way.

## Features
- **Health Check**: Verifies that the Tumblebug server is ready to handle requests before proceeding with operations.
- **Credential Registration**: Dynamically registers all valid credentials stored in a YAML file to the Tumblebug server.
- **Resource Loading**: Initiates the loading of common specs and images into Tumblebug.

## Prerequisites
- Python 3.7.5 or higher is installed
- Python packages listed in `requirements.txt`
- The `python3-venv` package should be installed for running the script using `init.sh`.

## Usage
### Direct Execution
```bash
pip3 install -r requirements.txt
```

```bash
python3 init.py
```

- Options: `-y, --yes` (Automatically answer yes to prompts and proceed without manual confirmation)

### Execution via Script
The `init.sh` script is provided to automate the setup of a Python virtual environment and running the `init.py` script. This is the recommended way to run the script.

Requires Python3.8 and above.

```bash
init.sh
```
- Options: `-y, --yes` (Automatically answer yes to prompts and proceed without manual confirmation)

## Configuration
Before running the script, ensure the following environment variables are set according to your Tumblebug server configuration:
- `TUMBLEBUG_SERVER`: The address of the Tumblebug server.
- `API_USERNAME`: Username for API authentication.
- `API_PASSWORD`: Password for API authentication.


## Related Files
- `init.py`: Main Python script.
- `requirements.txt`: Contains all Python dependencies.
- `init.sh`: Bash script for setting up a Python virtual environment and running `init.py`.
- `credentials.yaml`: Contains the credentials data to be registered with the Tumblebug server.
