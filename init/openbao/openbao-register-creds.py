#!/usr/bin/env python3
"""
MC-Terrarium CSP Credential Registration

Decrypts ~/.cloud-barista/credentials.yaml.enc and registers
CSP credentials into OpenBao KV v2 secret engine.

Designed for compatibility with cb-tumblebug's credential format,
enabling unified Cloud-Barista credential management.

Usage:
  uv run openbao-register-creds.py                      # Full init (interactive)
  uv run openbao-register-creds.py -y                   # Non-interactive
  uv run openbao-register-creds.py --key-file PATH      # Use key file for decryption
"""

import argparse

# import json
import os
import subprocess
import sys
import time
from getpass import getpass

import requests
import yaml
from colorama import Fore, Style
from colorama import init as colorama_init

# Initialize colorama
colorama_init(autoreset=True)

# ── Argument parsing ──────────────────────────────────────────────

parser = argparse.ArgumentParser(
    description="Initialize MC-Terrarium: OpenBao setup and CSP credential import.",
    formatter_class=argparse.RawDescriptionHelpFormatter,
    epilog="""
Examples:
  %(prog)s                                # Full initialization (interactive)
  %(prog)s -y                             # Non-interactive (auto-confirm)
  %(prog)s --key-file /path/to/keyfile    # Use key file for decryption
    """,
)
parser.add_argument(
    "-y",
    "--yes",
    action="store_true",
    help="Automatically proceed without confirmation prompts",
)
parser.add_argument(
    "--key-file",
    type=str,
    default=None,
    help="Path to decryption key file (default: ~/.cloud-barista/.tmp_enc_key, then prompt)",
)
parser.add_argument(
    "--env-file",
    type=str,
    default=".env",
    help="Path to .env file for OpenBao config (default: .env)",
)
args = parser.parse_args()

# ── Configuration ─────────────────────────────────────────────────

SCRIPT_DIR = os.path.dirname(os.path.abspath(__file__))


def get_project_root():
    """Find the project root directory by looking for .git or go.mod."""
    # Method 1: Try Git
    try:
        git_root = (
            subprocess.check_output(["git", "rev-parse", "--show-toplevel"], stderr=subprocess.STDOUT).decode().strip()
        )
        if os.path.isdir(git_root):
            return git_root
    except (subprocess.CalledProcessError, FileNotFoundError):
        pass

    # Method 2: Search upwards for markers
    curr_dir = os.path.abspath(SCRIPT_DIR)
    while curr_dir != os.path.dirname(curr_dir):
        if any(os.path.exists(os.path.join(curr_dir, marker)) for marker in [".git", "go.mod", "pyproject.toml"]):
            # Special case for openbao/pyproject.toml — keep searching if it's the current one
            if os.path.exists(os.path.join(curr_dir, "openbao-register-creds.py")):
                curr_dir = os.path.dirname(curr_dir)
                continue
            return curr_dir
        curr_dir = os.path.dirname(curr_dir)

    # Method 3: Fallback to fixed depth
    return os.path.dirname(os.path.dirname(os.path.dirname(SCRIPT_DIR)))


PROJECT_DIR = get_project_root()

VAULT_ADDR = os.getenv("VAULT_ADDR", "http://localhost:8200")
VAULT_TOKEN = os.getenv("VAULT_TOKEN", "")

# KV v2 path configuration
# Mount: "secret" (default KV v2 mount — OpenTofu/Terraform standard)
# Prefix: "csp" (logical namespace for CSP credentials)
# CLI usage:  bao kv get secret/csp/aws
# HCL usage:  vault_kv_secret_v2 { mount = "secret", name = "csp/aws" }
# API path:   /v1/secret/data/csp/{provider}  ("data" is KV v2 API convention)
KV_MOUNT = "secret"
SECRET_PREFIX = "csp"

CRED_PATH = os.path.join(os.path.expanduser("~"), ".cloud-barista")
ENC_FILE = os.path.join(CRED_PATH, "credentials.yaml.enc")
KEY_FILE = os.path.join(CRED_PATH, ".tmp_enc_key")

# CSP key mapping: cb-tumblebug YAML keys → Terrarium/OpenTofu env var keys
KEY_MAP = {
    "aws": {
        "aws_access_key_id": "AWS_ACCESS_KEY_ID",
        "aws_secret_access_key": "AWS_SECRET_ACCESS_KEY",
    },
    "azure": {
        "clientId": "ARM_CLIENT_ID",
        "clientSecret": "ARM_CLIENT_SECRET",
        "tenantId": "ARM_TENANT_ID",
        "subscriptionId": "ARM_SUBSCRIPTION_ID",
    },
    "gcp": {
        "project_id": "project_id",
        "client_email": "client_email",
        "private_key": "private_key",
        "private_key_id": "private_key_id",
        "client_id": "client_id",
        "S3AccessKey": "S3AccessKey",
        "S3SecretKey": "S3SecretKey",
    },
    "alibaba": {
        "AccessKeyId": "ALIBABA_CLOUD_ACCESS_KEY_ID",
        "AccessKeySecret": "ALIBABA_CLOUD_ACCESS_KEY_SECRET",
    },
    "ibm": {
        "ApiKey": "IC_API_KEY",
        "S3AccessKey": "S3_ACCESS_KEY",
        "S3SecretKey": "S3_SECRET_KEY",
    },
    "ncp": {
        "ncloud_access_key": "NCLOUD_ACCESS_KEY",
        "ncloud_secret_key": "NCLOUD_SECRET_KEY",
    },
    "tencent": {
        "SecretId": "TENCENTCLOUD_SECRET_ID",
        "SecretKey": "TENCENTCLOUD_SECRET_KEY",
    },
    "kt": {
        "IdentityEndpoint": "KT_IDENTITY_ENDPOINT",
        "Username": "KT_USERNAME",
        "Password": "KT_PASSWORD",
        "DomainName": "KT_DOMAIN_NAME",
        "ProjectID": "KT_PROJECT_ID",
        "S3AccessKey": "KT_S3_ACCESS_KEY",
        "S3SecretKey": "KT_S3_SECRET_KEY",
    },
    "nhn": {
        "IdentityEndpoint": "NHN_IDENTITY_ENDPOINT",
        "Username": "NHN_USERNAME",
        "Password": "NHN_PASSWORD",
        "DomainName": "NHN_DOMAIN_NAME",
        "TenantId": "NHN_TENANT_ID",
        "S3AccessKey": "NHN_S3_ACCESS_KEY",
        "S3SecretKey": "NHN_S3_SECRET_KEY",
    },
    "openstack": {
        "IdentityEndpoint": "OS_AUTH_URL",
        "Username": "OS_USERNAME",
        "Password": "OS_PASSWORD",
        "DomainName": "OS_DOMAIN_NAME",
        "ProjectID": "OS_PROJECT_ID",
    },
}


# ── Helper functions ──────────────────────────────────────────────


def load_env_file(path_arg):
    """Load VAULT_TOKEN and VAULT_ADDR from .env file, with fallback to docker-compose/.env."""
    global VAULT_TOKEN
    global VAULT_ADDR

    # Priority 1: Specified path (if exists)
    path = path_arg

    # Priority 2: If default ".env" not found, try deployments/docker-compose/.env
    if path == ".env" and not os.path.isfile(path):
        fallback_path = os.path.join("deployments", "docker-compose", ".env")
        if os.path.isfile(fallback_path):
            print(Fore.YELLOW + "Using fallback .env: " + fallback_path)
            path = fallback_path
        else:
            # Maybe we are running from inside deployments/docker-compose/openbao?
            # Try reaching up to project root .env
            up_path = os.path.join(PROJECT_DIR, ".env")
            if os.path.isfile(up_path):
                path = up_path
            else:
                # Try reaching up to deployments/docker-compose/.env
                up_fallback = os.path.join(PROJECT_DIR, "deployments", "docker-compose", ".env")
                if os.path.isfile(up_fallback):
                    path = up_fallback

    if os.path.isfile(path):
        with open(path) as f:
            for line in f:
                line = line.strip()
                if line.startswith("VAULT_TOKEN="):
                    VAULT_TOKEN = line.split("=", 1)[1].strip()
                    os.environ["VAULT_TOKEN"] = VAULT_TOKEN
                elif line.startswith("VAULT_ADDR="):
                    VAULT_ADDR = line.split("=", 1)[1].strip()
                    os.environ["VAULT_ADDR"] = VAULT_ADDR


def check_openbao_status():
    """Check OpenBao seal status. Returns (initialized, sealed) or exits on error."""
    try:
        resp = requests.get(f"{VAULT_ADDR}/v1/sys/seal-status", timeout=5)
        resp.raise_for_status()
        data = resp.json()
        return data["initialized"], data["sealed"]
    except requests.RequestException:
        print(Fore.RED + f"Cannot reach OpenBao at {VAULT_ADDR}.")
        print(Fore.YELLOW + "Please start and initialize OpenBao via your deployment scripts.")
        sys.exit(1)


def decrypt_credentials(enc_file_path, key):
    """Decrypt credentials.yaml.enc using openssl."""
    try:
        result = subprocess.run(
            ["openssl", "enc", "-aes-256-cbc", "-d", "-pbkdf2", "-in", enc_file_path, "-pass", f"pass:{key}"],
            capture_output=True,
            check=True,
        )
        return result.stdout.decode("utf-8"), None
    except subprocess.CalledProcessError as e:
        return None, f"Decryption error: {e.stderr.decode('utf-8').strip()}"


def get_decrypted_content():
    """Decrypt credentials file using key file or interactive password.

    Resolution order (same as cb-tumblebug):
      1. --key-file <path>  (explicit CLI argument)
      2. ~/.cloud-barista/.tmp_enc_key  (convention)
      3. Interactive password prompt (up to 3 attempts)
    """
    # 1. Try explicit --key-file argument
    if args.key_file and os.path.isfile(args.key_file):
        with open(args.key_file) as kf:
            key = kf.read().strip()
        print(Fore.YELLOW + f"Using key from {args.key_file}")
        content, error = decrypt_credentials(ENC_FILE, key)
        if error is None:
            return content, True  # (content, used_key_file)
        print(Fore.RED + error)

    # 2. Try default .tmp_enc_key (cb-tumblebug convention)
    if os.path.isfile(KEY_FILE):
        with open(KEY_FILE) as kf:
            key = kf.read().strip()
        print(Fore.YELLOW + f"Using key from {KEY_FILE}")
        content, error = decrypt_credentials(ENC_FILE, key)
        if error is None:
            return content, True  # (content, used_key_file)
        print(Fore.RED + error)

    # 3. Check Environment Variable (MULTI_INIT_PWD)
    env_password = os.environ.get("MULTI_INIT_PWD")
    if env_password:
        content, error = decrypt_credentials(ENC_FILE, env_password)
        if error is None:
            return content, False  # (content, used_key_file=False)
        else:
            print(Fore.YELLOW + "Warning: Password in MULTI_INIT_PWD failed to decrypt. Falling back to manual prompt.")

    # 4. Prompt for password (up to 3 attempts)
    for attempt in range(1, 4):
        password = getpass(f"Enter the password for credentials.yaml.enc (attempt {attempt}/3): ")
        content, error = decrypt_credentials(ENC_FILE, password)
        if error is None:
            return content, False  # (content, used_key_file=False)
        print(Fore.RED + error)

    print(Fore.RED + "Failed to decrypt after 3 attempts. Exiting.")
    sys.exit(1)


def register_credential(holder, provider, credentials):
    """Register a single CSP credential to OpenBao for a specific holder."""
    # Check if provider has any non-empty values
    has_value = any(v for v in credentials.values() if v)
    if not has_value:
        return provider, "skip", "No credential values"

    # Build secret data with Terrarium-compatible keys
    secret_data = {}
    key_map = KEY_MAP.get(provider, {})
    mapped_keys = []
    placeholder_keys = []

    if key_map:
        # Map existing values and fill missing with placeholders from KEY_MAP
        for yaml_key, terrarium_key in key_map.items():
            value = credentials.get(yaml_key)
            if value:
                secret_data[terrarium_key] = value
                mapped_keys.append(terrarium_key)
            else:
                secret_data[terrarium_key] = ""
                placeholder_keys.append(terrarium_key)
    else:
        # No mapping found — store raw keys as-is
        for yaml_key, value in credentials.items():
            if value:
                secret_data[yaml_key] = value
                mapped_keys.append(yaml_key)
            else:
                secret_data[yaml_key] = ""
                placeholder_keys.append(yaml_key)

    if not mapped_keys and not placeholder_keys:
        return provider, "skip", "No keys to register"

    # Determine secret prefix based on holder
    # - 'admin' -> csp/
    # - others  -> users/{holder}/csp/
    if holder == "admin":
        prefix = SECRET_PREFIX
    else:
        prefix = f"users/{holder}/{SECRET_PREFIX}"

    # Register to OpenBao via KV v2 API
    # KV v2 write path: /v1/{mount}/data/{prefix}/{name}
    url = f"{VAULT_ADDR}/v1/{KV_MOUNT}/data/{prefix}/{provider}"
    headers = {
        "X-Vault-Token": VAULT_TOKEN,
        "Content-Type": "application/json",
    }
    try:
        resp = requests.post(url, json={"data": secret_data}, headers=headers, timeout=10)
        resp.raise_for_status()
        version = resp.json().get("data", {}).get("version", "?")

        total_keys = len(mapped_keys) + len(placeholder_keys)
        if placeholder_keys:
            p_msg = f"including {len(placeholder_keys)} placeholder(s)"
            return provider, "ok", f"v{version}  ({total_keys} keys, {p_msg})"
        return provider, "ok", f"v{version}  ({total_keys} keys)"
    except requests.RequestException as e:
        return provider, "fail", str(e).split(":")[0]


def register_placeholder_secrets(registered_providers):
    """Register placeholder secrets for CSPs not present in the credential file.

    This ensures vault_kv_secret_v2 data sources do not hard-fail during
    tofu plan/apply when a CSP's credentials have not been provided yet.
    Providers will receive empty strings and fail gracefully at auth time
    rather than crashing the entire plan.
    """
    headers = {
        "X-Vault-Token": VAULT_TOKEN,
        "Content-Type": "application/json",
    }
    placeholder_count = 0

    for provider, key_mapping in KEY_MAP.items():
        if provider in registered_providers:
            continue

        # Check if secret already exists in OpenBao
        url = f"{VAULT_ADDR}/v1/{KV_MOUNT}/data/{SECRET_PREFIX}/{provider}"
        try:
            resp = requests.get(url, headers=headers, timeout=5)
            if resp.status_code == 200:
                # Secret already exists (registered outside this script)
                continue
        except requests.RequestException:
            pass

        # Build placeholder data with empty strings for all expected keys
        placeholder_data = {v: "" for v in key_mapping.values()}
        try:
            resp = requests.post(url, json={"data": placeholder_data}, headers=headers, timeout=10)
            resp.raise_for_status()
            print(
                f"  {Fore.YELLOW}PLCH{Style.RESET_ALL} {provider:12s}  placeholder registered "
                f"({len(placeholder_data)} keys, all placeholders)"
            )
            placeholder_count += 1
        except requests.RequestException as e:
            print(f"  {Fore.RED}FAIL{Style.RESET_ALL} {provider:12s}  placeholder failed: {str(e).split(':')[0]}")

    return placeholder_count


# ── Main ──────────────────────────────────────────────────────────


def main():
    global VAULT_TOKEN

    print(Style.BRIGHT + Fore.CYAN)
    print("=" * 60)
    print("  MC-Terrarium CSP Credential Registration")
    print("=" * 60)
    print(Style.RESET_ALL)

    # Show configuration
    print(Fore.YELLOW + "Configuration")
    print(f" - {Fore.CYAN}VAULT_ADDR:{Fore.RESET} {VAULT_ADDR}")
    print(f" - {Fore.CYAN}CRED_FILE:{Fore.RESET} {ENC_FILE}")
    print(f" - {Fore.CYAN}KEY_FILE:{Fore.RESET} {KEY_FILE}")
    print()

    # ── Step 1: OpenBao Status Check ──────────────────────────────
    # Load .env variables first
    load_env_file(args.env_file)

    print(Style.BRIGHT + "── OpenBao Initialization Check ──" + Style.RESET_ALL)
    initialized, sealed = check_openbao_status()

    if not initialized:
        print(Fore.RED + "OpenBao is not yet initialized.")
        print(Fore.YELLOW + "Please run your deployment's openbao-init.sh first.")
        sys.exit(1)
    elif sealed:
        print(Fore.RED + "OpenBao is sealed.")
        print(Fore.YELLOW + "Please run your deployment's openbao-unseal.sh first.")
        sys.exit(1)
    else:
        print(Fore.GREEN + "OpenBao is initialized and unsealed. Ready.")
    print()

    # ── Step 2: Register CSP credentials ────────────────────────────
    if os.path.isfile(ENC_FILE):
        print(Style.BRIGHT + "── CSP Credential Registration ──" + Style.RESET_ALL)

        # Ensure VAULT_TOKEN is available
        if not VAULT_TOKEN:
            load_env_file(args.env_file)
        if not VAULT_TOKEN:
            print(Fore.RED + "VAULT_TOKEN not set. Ensure .env exists in project root or deployments/docker-compose/.")
            sys.exit(1)

        # Check OpenBao is ready
        initialized, sealed = check_openbao_status()
        if not initialized:
            print(Fore.RED + "OpenBao is not initialized. Run openbao-init.sh first.")
            sys.exit(1)
        if sealed:
            print(Fore.RED + "OpenBao is sealed. Run openbao-unseal.sh first.")
            sys.exit(1)

        # Determine if password input will be required (tumblebug pattern)
        # If password is needed, it serves as confirmation (skip "proceed?" prompt)
        password_required = not args.key_file and not os.path.isfile(KEY_FILE)

        # Decrypt credentials BEFORE proceed (tumblebug pattern)
        if password_required:
            print(Fore.CYAN + "Enter the credential password to proceed...")
        print(Fore.CYAN + "Decrypting credentials...")
        decrypted_content, used_key_file = get_decrypted_content()
        print(Fore.GREEN + "Decryption successful!")
        print()

        # If key file was used (no password prompt), ask for confirmation
        if used_key_file and not args.yes:
            confirm = input(Fore.CYAN + "Proceed? (y/n): " + Style.RESET_ALL).lower()
            if confirm not in ("y", "yes"):
                print(Fore.GREEN + "Cancelled.")
                sys.exit(0)
            print()

        start_time = time.time()

        # Parse YAML
        try:
            data = yaml.safe_load(decrypted_content)
            cred_holders = data.get("credentialholder", {})
            if not cred_holders:
                print(Fore.RED + "No 'credentialholder' found in credentials YAML.")
                sys.exit(1)
        except Exception:
            print(Fore.RED + "Error parsing credentials YAML. Ensure the format is correct.")
            sys.exit(1)

        # Register each CSP
        print(Fore.CYAN + "Registering credentials to OpenBao...")
        print()

        success_count = 0
        skip_count = 0
        fail_count = 0
        registered_providers = set()

        # Keep track of registered paths to identify where placeholders are needed for 'admin'
        admin_registered = set()

        for holder, holder_creds in cred_holders.items():
            print(f" Holder: {Fore.MAGENTA}{holder}{Style.RESET_ALL}")
            for provider, credentials in holder_creds.items():
                provider_name, status, message = register_credential(holder, provider, credentials)
                if status == "ok":
                    print(f"  {Fore.GREEN}OK  {Style.RESET_ALL} {provider_name:12s}  {message}")
                    success_count += 1
                    if holder == "admin":
                        admin_registered.add(provider_name)
                    registered_providers.add(f"{holder}/{provider_name}")
                elif status == "skip":
                    print(f"  {Fore.YELLOW}SKIP{Style.RESET_ALL} {provider_name:12s}  ({message})")
                    skip_count += 1
                else:
                    print(f"  {Fore.RED}FAIL{Style.RESET_ALL} {provider_name:12s}  {message}")
                    fail_count += 1
            print()

        # Register placeholder secrets for CSPs not in the 'admin' credential set.
        # This ensures basic 'tofu plan' works without full credentials.
        placeholder_count = register_placeholder_secrets(admin_registered)

        print()
        print(
            f"Results: {Fore.GREEN}{success_count} registered{Style.RESET_ALL}, "
            f"{Fore.YELLOW}{skip_count} skipped{Style.RESET_ALL}, "
            f"{Fore.RED}{fail_count} failed{Style.RESET_ALL}"
            + (f", {Fore.CYAN}{placeholder_count} placeholders{Style.RESET_ALL}" if placeholder_count > 0 else "")
        )

        if fail_count > 0:
            print(Fore.RED + "\nSome credentials failed to register.")

        # Summary
        elapsed = int(time.time() - start_time)
        print(Style.BRIGHT + Fore.GREEN)
        print("=" * 60)
        print(f"  Initialization complete! ({elapsed}s)")
        print("=" * 60)
        print(Style.RESET_ALL)
    else:
        print(Fore.YELLOW + f"Skipping: {ENC_FILE} not found.")
        print(Fore.YELLOW + "Generate it using cb-tumblebug/init/encCredential.sh")

    print()

    # Usage hint
    print("To verify a credential:")
    print("  source .env")
    print(
        f'  curl -s -H "X-Vault-Token: $$VAULT_TOKEN" {VAULT_ADDR}/v1/{KV_MOUNT}/data/{SECRET_PREFIX}/aws '
        "| jq .data.data"
    )
    print(f"  bao kv get {KV_MOUNT}/{SECRET_PREFIX}/aws")
    print()


if __name__ == "__main__":
    main()
