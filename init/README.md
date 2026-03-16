# README.md for `init.py`

## Overview

The `init.py` script is designed to automate the process of registering credentials and loading common specifications and images for a Tumblebug server. It can be executed directly or via the `init.sh` script which sets up a Python virtual environment. This script ensures the Tumblebug server is healthy before proceeding and performs several network operations in a secure and managed way.

### What `make init` Does

`make init` runs `init.sh` → `init.py`, which performs two independent credential registration paths:

1. **Tumblebug → CB-Spider**: Registers cloud credentials via the Tumblebug API (hybrid-encrypted with RSA + AES)
2. **OpenBao (← MC-Terrarium)**: Registers CSP credentials into OpenBao KV v2 (`secret/csp/{provider}`); MC-Terrarium's OpenTofu templates query OpenBao at runtime

Both paths read from the same encrypted credential file (`~/.cloud-barista/credentials.yaml.enc`).

```
~/.cloud-barista/credentials.yaml.enc
        ↓ init.py (decrypt in-memory)
        ├─→ Tumblebug API → CB-Spider (cloud connections)
        └─→ OpenBao KV v2 (← Terrarium's OpenTofu templates)
```

## 🚀 NEW: Fast Initialization with Database Backup

Starting from this version, `init.py` supports **fast initialization** using
pre-built database backups:

- **Standard mode** (~20 minutes): Fetches fresh data from all Cloud Service
  Provider APIs
- **Fast mode** (~1 minute): Restores from pre-built database backup in
  `assets/assets.dump.gz`

When running `init.py`, you'll be prompted:

```text
🚀 Database Backup Found!
========================================

A pre-built database backup was found:
  Location: ./assets/assets.dump.gz
  Size:     74 MB

  ✅ Fast initialization (restore from backup): ~1 minute
  ⏱️  Standard initialization (fetch from CSPs):  ~20 minutes

Would you like to use the backup database? (y/n):
```

**Auto-yes mode**: Use `-y` flag to automatically use backup without prompting:

```bash
cd ./init && uv run init.py -y
```

## Features

- **Health Check**: Verifies that the Tumblebug server is ready to handle requests before proceeding with operations.
- **Credential Registration**: Dynamically registers all valid credentials stored in a YAML file to the Tumblebug server.
- **Fast Database Restore** (**NEW**): Optionally restore from pre-built
  database backup (~1 min vs ~20 min)
- **Resource Loading**: Initiates the loading of common specs and images into
  Tumblebug from CSP APIs.

## Prerequisites

- Python 3.8.0 or higher is installed
- uv 0.6.16 or higher is installed
  - uv is an emerging Python package and project manager
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
- `init-openbao.sh`: One-time OpenBao initialization (generates unseal key + root token).
- `unseal-openbao.sh`: Unseals OpenBao after container restart.

> For OpenBao auto-initialization, credential paths, and Makefile targets, see [Appendix](#appendix-openbao-reference) below.

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

---

## Appendix: OpenBao Reference

### How Auto-Initialization Works

1. `make up` starts the OpenBao container first
2. If `VAULT_TOKEN` is not set in `.env`, runs `init-openbao.sh` to:
   - Initialize OpenBao (1 unseal key, threshold 1)
   - Save unseal key and root token to `secrets/openbao-init.json`
   - Write `VAULT_TOKEN` to `.env`
3. Unseals OpenBao using `unseal-openbao.sh`
4. Starts all remaining services

On subsequent restarts (`make up`), only the unseal step runs — no re-initialization.

### OpenBao Credential Paths

| CSP       | Path                   | Key Names                                                                      |
| --------- | ---------------------- | ------------------------------------------------------------------------------ |
| AWS       | `secret/csp/aws`       | `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`                                   |
| GCP       | `secret/csp/gcp`       | `project_id`, `client_email`, `private_key`, `private_key_id`, `client_id`     |
| Azure     | `secret/csp/azure`     | `ARM_CLIENT_ID`, `ARM_CLIENT_SECRET`, `ARM_TENANT_ID`, `ARM_SUBSCRIPTION_ID`   |
| Alibaba   | `secret/csp/alibaba`   | `ALIBABA_CLOUD_ACCESS_KEY_ID`, `ALIBABA_CLOUD_ACCESS_KEY_SECRET`               |
| IBM       | `secret/csp/ibm`       | `IC_API_KEY`                                                                   |
| NCP       | `secret/csp/ncp`       | `NCLOUD_ACCESS_KEY`, `NCLOUD_SECRET_KEY`                                       |
| Tencent   | `secret/csp/tencent`   | `TENCENTCLOUD_SECRET_ID`, `TENCENTCLOUD_SECRET_KEY`                            |
| OpenStack | `secret/csp/openstack` | `OS_AUTH_URL`, `OS_USERNAME`, `OS_PASSWORD`, `OS_DOMAIN_NAME`, `OS_PROJECT_ID` |

### Makefile Targets (OpenBao)

| Target              | Description                                               |
| ------------------- | --------------------------------------------------------- |
| `make up`           | Start all services (auto-init/unseal OpenBao)             |
| `make down`         | Stop all services                                         |
| `make init`         | Register credentials to both Tumblebug and OpenBao        |
| `make unseal`       | Manually unseal OpenBao                                   |
| `make init-openbao` | Manually initialize OpenBao (first run only)              |
| `make clean-db`     | Delete Tumblebug/Spider/Terrarium data (keeps OpenBao)    |
| `make clean-all`    | Full reset including OpenBao (requires `make init` again) |

> For troubleshooting and more details, see [MC-Terrarium v0.1.0 — init/README.md](https://github.com/cloud-barista/mc-terrarium/blob/v0.1.0/init/README.md).
