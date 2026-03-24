# OpenBao Initialization Secrets

This directory is used to store sensitive outputs from the **OpenBao** initialization process.

## Root vs Component Secrets

Currently, there are two `secrets/` directories in this project:

- **[`secrets/`](../../../secrets/) (Root)**: The primary location for initialization outputs.
- **[`init/openbao/secrets/`](./)** (This directory): A secondary location for component-specific placeholders or documentation.

## Key Files

| File                | Description                                                                                                     |
| :------------------ | :-------------------------------------------------------------------------------------------------------------- |
| `openbao-init.json` | Contains the **unseal key** and **root token** for OpenBao. Required for unsealing and credential registration. |
| `.gitkeep`          | Preserves this directory in Git even when empty.                                                                |
| `README.md`         | This file (unignored project-wide in `.gitignore`).                                                             |

## Management Scripts

The files in this directory are typically generated or consumed by the scripts in the parent directory:

- **[`../openbao-init.sh`](../openbao-init.sh)**: Generates `openbao-init.json`.
- **[`../openbao-unseal.sh`](../openbao-unseal.sh)**: Reads `openbao-init.json` to unseal the server.

---

> [!CAUTION]
> **NEVER** commit `openbao-init.json` or other sensitive secret files to version control.
> They are excluded globally by `**/secrets/*` in `.gitignore`.
