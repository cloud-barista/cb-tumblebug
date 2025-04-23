
# README.md for `init.py`

## Overview
The `init.py` script is designed to automate the process of registering credentials and loading common specifications and images for a Tumblebug server. It can be executed directly or via the `init.sh` script which sets up a Python virtual environment. This script ensures the Tumblebug server is healthy before proceeding and performs several network operations in a secure and managed way.

## Features
- **Health Check**: Verifies that the Tumblebug server is ready to handle requests before proceeding with operations.
- **Credential Registration**: Dynamically registers all valid credentials stored in a YAML file to the Tumblebug server.
- **Resource Loading**: Initiates the loading of common specs and images into Tumblebug.

## Prerequisites
- Python 3.8.0 or higher is installed
- uv 0.6.16 or higher is installed
- Python packages listed in `pyproject.toml`

### `uv` installation

See [Installing uv](https://docs.astral.sh/uv/getting-started/installation/)

```shell
# Installing uv
curl -LsSf https://astral.sh/uv/install.sh | sh

# Setting environment variables
source ~/.bashrc
# or source ~/.bash_profile, source ~/.profile 
```

Note: Removing uv is described at the last section.

## Usage

### Encrypting Credentials
Before running `init.py`, you must encrypt your `credentials.yaml` file to ensure the security of your sensitive information.

1. Use the `encCredential.sh` script to encrypt your `credentials.yaml` file:

```bash
init/encCredential.sh
```

The `init.py` script will decrypt the `credentials.yaml.enc` file as needed to read the credentials. You may need to provide a password if the decryption key is not stored.


### Direct Execution
```bash
uv run init.py
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
- `TB_API_USERNAME`: Username for API authentication.
- `TB_API_PASSWORD`: Password for API authentication.

## Security Considerations
To protect sensitive information, `credentials.yaml` is not used directly. Instead, it must be encrypted using `encCredential.sh`. The encrypted file `credentials.yaml.enc` is then used by `init.py`. This approach ensures that sensitive credentials are not stored in plain text.

If you need to update your credentials, decrypt the encrypted file using `decCredential.sh`, make the necessary changes to `credentials.yaml`, and then re-encrypt it.

### Encrypting Credentials
```bash
init/encCredential.sh
```

### Decrypting Credentials
```bash
init/decCredential.sh
```

## Related Files
- `init.py`: Main Python script.
- `pyproject.toml`: Contains all Python dependencies.
- `init.sh`: Bash script for setting up a Python virtual environment and running `init.py`.
- `credentials.yaml`: Contains the credentials data to be registered with the Tumblebug server.
- `encCredential.sh`: Script to encrypt `credentials.yaml`.
- `decCredential.sh`: Script to decrypt `credentials.yaml.enc`.

---

## Note: Removing uv

1. Clean up stored data (optional):

```shell
uv cache clean
rm -r "$(uv python dir)"
rm -r "$(uv tool dir)"
```

2. Remove the uv and uvx binaries:

```shell
rm ~/.local/bin/uv ~/.local/bin/uvx
```
