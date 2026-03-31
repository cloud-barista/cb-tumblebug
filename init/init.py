#!/usr/bin/env python3

import argparse
import base64
import os
import subprocess
import sys
import threading
import time
from concurrent.futures import ThreadPoolExecutor, as_completed
from getpass import getpass

import requests
import yaml
from alive_progress import alive_bar
from colorama import Fore, Style, init
from Crypto.Cipher import AES, PKCS1_OAEP
from Crypto.Hash import SHA256
from Crypto.PublicKey import RSA
from Crypto.Random import get_random_bytes
from Crypto.Util.Padding import pad
from tabulate import tabulate

parser = argparse.ArgumentParser(
    description="Initialize CB-Tumblebug with credentials, assets, and pricing information.",
    formatter_class=argparse.RawDescriptionHelpFormatter,
    epilog="""
Examples:
  %(prog)s                                    # Run all steps (default)
  %(prog)s -y                                 # Run all steps without confirmation
  %(prog)s --credentials-only                 # Register credentials only
  %(prog)s --load-assets-only                 # Load assets (specs and images) only
  %(prog)s --fetch-price-only                 # Fetch price information only
  %(prog)s --load-templates-only              # Load template files only
  %(prog)s --credentials --load-assets        # Register credentials and load assets
  %(prog)s -y --credentials --fetch-price     # Register credentials and fetch price (no confirmation)
  %(prog)s --key-file /path/to/keyfile        # Use key file for decryption
    """,
)
parser.add_argument("-y", "--yes", action="store_true", help="Automatically answer yes to prompts and proceed without confirmation")
parser.add_argument(
    "--credentials",
    "--credentials-only",
    action="store_true",
    dest="credentials_only",
    help="Register cloud credentials only",
)
parser.add_argument(
    "--load-assets",
    "--load-assets-only",
    action="store_true",
    dest="load_assets_only",
    help="Load common specs and images only",
)
parser.add_argument(
    "--fetch-price",
    "--fetch-price-only",
    action="store_true",
    dest="fetch_price_only",
    help="Fetch price information only",
)
parser.add_argument(
    "--load-templates",
    "--load-templates-only",
    action="store_true",
    dest="load_templates_only",
    help="Load template files from init/templates/ directory",
)
parser.add_argument(
    "--key-file",
    type=str,
    default=None,
    help="Path to decryption key file (default: ~/.cloud-barista/.tmp_enc_key, then prompt)",
)
args = parser.parse_args()

# Determine which operations to run
# If no specific options are provided, run all operations (default behavior)
run_all = not (args.credentials_only or args.load_assets_only or args.fetch_price_only or args.load_templates_only)
run_credentials = run_all or args.credentials_only
run_load_assets = run_all or args.load_assets_only
run_fetch_price = run_all or args.fetch_price_only
run_load_templates = run_all or args.load_templates_only

# Initialize colorama
init(autoreset=True)

# Configuration
TUMBLEBUG_SERVER = os.getenv("TUMBLEBUG_SERVER", "localhost:1323")
TB_API_USERNAME = os.getenv("TB_API_USERNAME", "default")
TB_API_PASSWORD = os.getenv("TB_API_PASSWORD", "default")
AUTH = f"Basic {base64.b64encode(f'{TB_API_USERNAME}:{TB_API_PASSWORD}'.encode()).decode()}"
HEADERS = {"Authorization": AUTH, "Content-Type": "application/json"}

CRED_FILE_NAME_ENC = "credentials.yaml.enc"
CRED_PATH = os.path.join(os.path.expanduser("~"), ".cloud-barista")
ENC_FILE_PATH = os.path.join(CRED_PATH, CRED_FILE_NAME_ENC)
KEY_FILE = os.path.join(CRED_PATH, ".tmp_enc_key")

expected_completion_time_seconds = 400  # Default 400 seconds for non-Azure asset load

# ══════════════════════════════════════════════════════════════════════════════

# Check for credential path
if not os.path.exists(CRED_PATH):
    print(Fore.RED + "Error: CRED_PATH does not exist. Please run init/genCredential.sh first.")
    sys.exit(1)
elif not os.path.isfile(ENC_FILE_PATH):
    print(Fore.RED + f"Error: {CRED_FILE_NAME_ENC} does not exist. Please check if it has been generated.")
    print(Fore.RED + f"- This script does not accept 'credentials.yaml'. For your security, it only accepts an encrypted file.")
    print(Fore.RED + f"- Please generate '{CRED_FILE_NAME_ENC}' using 'init/encCredential.sh'.")
    sys.exit(1)


# Decrypt credentials.yaml.enc
def decrypt_credentials(enc_file_path, key):
    try:
        result = subprocess.run(
            ["openssl", "enc", "-aes-256-cbc", "-d", "-pbkdf2", "-in", enc_file_path, "-pass", f"pass:{key}"],
            check=True,
            capture_output=True,
        )
        if result.returncode != 0:
            return None, "Decryption failed."
        return result.stdout.decode("utf-8"), None
    except subprocess.CalledProcessError as e:
        return None, f"Decryption error: {e.stderr.decode('utf-8')}"


def get_decryption_key():
    """Decrypt credentials file using key file or interactive password.

    Resolution order (same as mc-terrarium):
      1. --key-file <path>  (explicit CLI argument)
      2. ~/.cloud-barista/.tmp_enc_key  (convention)
      3. Interactive password prompt (up to 3 attempts)
    """
    # 1. Try explicit --key-file argument
    if args.key_file and os.path.isfile(args.key_file):
        with open(args.key_file, "r") as kf:
            key = kf.read().strip()
        print(Fore.YELLOW + f"Using key from {args.key_file}")
        decrypted_content, error = decrypt_credentials(ENC_FILE_PATH, key)
        if error is None:
            return decrypted_content
        print(Fore.RED + error)

    # 2. Try default .tmp_enc_key (cb-tumblebug convention)
    if os.path.isfile(KEY_FILE):
        with open(KEY_FILE, "r") as kf:
            key = kf.read().strip()
        print(Fore.YELLOW + f"Using key from {KEY_FILE}")
        decrypted_content, error = decrypt_credentials(ENC_FILE_PATH, key)
        if error is None:
            return decrypted_content
        print(Fore.RED + error)

    # 3. Check Environment Variable (MULTI_INIT_PWD)
    env_password = os.environ.get("MULTI_INIT_PWD")
    if env_password:
        decrypted_content, error = decrypt_credentials(ENC_FILE_PATH, env_password)
        if error is None:
            return decrypted_content
        else:
            print(Fore.YELLOW + "Warning: Password in MULTI_INIT_PWD failed to decrypt. Falling back to manual prompt.")

    # 4. Prompt for password (up to 3 attempts)
    for attempt in range(3):
        password = getpass(f"Enter the password of the encrypted credential to continue (attempt {attempt + 1}/3): ")
        decrypted_content, error = decrypt_credentials(ENC_FILE_PATH, password)
        if error is None:
            return decrypted_content
        print(Fore.RED + error)

    print(Fore.RED + "Failed to decrypt the file after 3 attempts. Exiting.")
    sys.exit(1)


# Print the current configuration
print(Fore.YELLOW + "Current Configuration\nPlease set the corresponding environment variables to make changes.")
print(" - " + Fore.CYAN + "TUMBLEBUG_SERVER:" + Fore.RESET + f" {TUMBLEBUG_SERVER}")
print(" - " + Fore.CYAN + "TB_API_USERNAME:" + Fore.RESET + f" {TB_API_USERNAME[0]}**********")
print(" - " + Fore.CYAN + "TB_API_PASSWORD:" + Fore.RESET + f" {TB_API_PASSWORD[0]}**********")
print(" - " + Fore.CYAN + "CRED_PATH:" + Fore.RESET + f" {CRED_PATH}")
print(" - " + Fore.CYAN + "CRED_FILE_NAME:" + Fore.RESET + f" {CRED_FILE_NAME_ENC}")
print(" - " + Fore.CYAN + "expected completion time:" + Fore.RESET + f" {expected_completion_time_seconds} seconds\n")

# (Moved) server health check will run after user confirmation to ensure inputs are provided first

# Check for database backup availability early (before asking for confirmation)
backup_db_path = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "assets", "assets.dump.gz")
backup_available = os.path.isfile(backup_db_path)
backup_size_mb = 0
backup_age_days = 0
use_backup = False  # Decision variable
include_azure = False  # Decision variable for Azure image fetch

if backup_available:
    backup_size_mb = os.path.getsize(backup_db_path) / (1024 * 1024)

    # Get backup age from Git commit date (more accurate than file mtime)
    try:
        # Using git from trusted system path, file path is validated above
        git_commit_time_result = subprocess.run(
            ["git", "log", "-1", "--format=%ct", "--", "assets/assets.dump.gz"],
            cwd=os.path.dirname(os.path.abspath(__file__)) + "/..",
            capture_output=True,
            text=True,
            timeout=5,
            check=False,  # Don't raise exception on non-zero exit
        )
        if git_commit_time_result.returncode == 0 and git_commit_time_result.stdout.strip():
            git_commit_timestamp = int(git_commit_time_result.stdout.strip())
            backup_age_days = int((time.time() - git_commit_timestamp) / 86400)
        else:
            # Fallback to file mtime if git command fails
            backup_mtime = os.path.getmtime(backup_db_path)
            backup_age_days = int((time.time() - backup_mtime) / 86400)
    except (subprocess.TimeoutExpired, subprocess.CalledProcessError, ValueError):
        # Fallback to file mtime if git is not available or command fails
        backup_mtime = os.path.getmtime(backup_db_path)
        backup_age_days = int((time.time() - backup_mtime) / 86400)

# Ask for backup choice BEFORE showing operations summary
if run_load_assets and backup_available and not args.yes:
    print(Fore.CYAN + "=" * 80)
    print(Fore.CYAN + "🚀 Choose Initialization Method")
    print(Fore.CYAN + "=" * 80)
    print("")
    print(Fore.GREEN + "  a. ⏩ Restore from backup (~1 minute)")
    print(Fore.WHITE + f"     Location: ./assets/assets.dump.gz ({backup_size_mb:.1f} MB, {backup_age_days} days old)")
    if backup_age_days > 30:
        print(Fore.YELLOW + f"     ⚠️  Backup is {backup_age_days} days old - data may be outdated")
    elif backup_age_days > 7:
        print(Fore.YELLOW + f"     ℹ️  Backup is {backup_age_days} days old - consider fetching fresh data for latest info")
    print(Fore.WHITE + "     Contains: Specs, Images, and Pricing data")
    print(Fore.WHITE + "     → Steps 2 & 3 will be skipped")
    print("")
    print(Fore.YELLOW + "  b. 🔄 Fetch fresh from CSPs (~10-20 minutes)")
    print(Fore.WHITE + "     → Fetches latest specs, images from cloud providers (excluding Azure)")
    print(Fore.WHITE + "     → Step 3 (pricing) will run separately if requested")
    print("")
    print(Fore.MAGENTA + "  c. 🔁 Fetch from ALL CSPs including Azure (~40+ minutes)")
    print(Fore.WHITE + "     → Fetches from ALL cloud providers including Azure")
    print(Fore.YELLOW + "     ⚠️  Warning: Azure image fetch is very slow and may take 40+ minutes")
    print("")

    while True:
        choice = input(Fore.CYAN + "Select option (a/b/c): " + Fore.RESET).lower()
        if choice in ["a"]:
            use_backup = True
            include_azure = False
            break
        elif choice in ["b"]:
            use_backup = False
            include_azure = False
            break
        elif choice in ["c"]:
            use_backup = False
            include_azure = True
            break
        else:
            print(Fore.RED + "Invalid input. Please enter 'a', 'b', or 'c'.")
    print("")
elif run_load_assets and backup_available and args.yes:
    # Auto-yes mode: use backup by default
    use_backup = True
    include_azure = False
    print(Fore.GREEN + "\nAuto-yes mode: Using backup (Option A)." + Fore.RESET)
elif run_load_assets and not backup_available and not args.yes:
    # No backup available, ask about Azure
    print(Fore.YELLOW + "\n⚠️  No backup found. Image fetch from CSPs is required.")
    print(Fore.CYAN + "\nInclude Azure images?")
    print(Fore.WHITE + "  - No (default): ~20-30 minutes")
    print(Fore.YELLOW + "  - Yes: ~40+ minutes (Azure is very slow)")
    print("")

    while True:
        choice = input(Fore.CYAN + "Include Azure? (y/N): " + Fore.RESET).lower()
        if choice in ["y", "yes"]:
            include_azure = True
            break
        elif choice in ["n", "no", ""]:
            include_azure = False
            break
        else:
            print(Fore.RED + "Invalid input. Please enter 'y' or 'n'.")
    print("")

# Display what will be executed (after user choice)
operations = []
if run_credentials:
    operations.append("Register credentials → Tumblebug")
if run_load_assets:
    if use_backup:
        operations.append(f"Load assets from backup ({backup_size_mb:.1f} MB - includes specs, images, pricing)")
    else:
        operations.append("Load assets (fetch from CSPs)")
if run_fetch_price and not use_backup:
    operations.append("Fetch price information")

print(Fore.YELLOW + "\n" + "=" * 80)
print(Fore.YELLOW + "Operations to be performed:")
print(Fore.YELLOW + "=" * 80)
for i, op in enumerate(operations, 1):
    print(Fore.CYAN + f"  {i}. {op}")

if use_backup:
    print("")
    print(Fore.GREEN + "  ℹ️  Using backup - Steps 2 & 3 completed in ~1 minute")
elif run_load_assets and not backup_available:
    print("")
    print(Fore.YELLOW + "  ⚠️  No backup found - will fetch from CSPs (~20 minutes)")

print(Fore.YELLOW + "=" * 80)
print("")

# Determine if password input will be required
# If password is needed, it serves as confirmation (skip "proceed?" prompt)
# Decryption is needed for Tumblebug credentials.
needs_decryption = run_credentials
password_required = needs_decryption and not os.path.isfile(KEY_FILE)

# Get decryption key BEFORE health check (if credentials are being registered)
# This ensures all user inputs are completed before waiting for server
decrypted_content = None
all_holders = None

if needs_decryption:
    if password_required:
        # Password input serves as confirmation - no need for separate "proceed?" prompt
        print(Fore.CYAN + "Enter the credential password to proceed...")
    decrypted_content = get_decryption_key()
    all_holders = yaml.safe_load(decrypted_content)["credentialholder"]
    # all_holders is a dict: {"admin": {provider: creds, ...}, "role01": {provider: creds, ...}, ...}
else:
    # No credentials to register - ask for confirmation
    if not args.yes:
        if input(Fore.CYAN + "Do you want to proceed? (y/n): ").lower() not in ["y", "yes"]:
            print(Fore.GREEN + "Cancel [{}]".format(" ".join(sys.argv)))
            print(Fore.GREEN + "See you soon. :)")
            sys.exit(0)
    else:
        print(Fore.GREEN + "Auto-yes mode enabled - proceeding with selected options...")
        print("")

# If password was not required but credentials are being registered, ask for confirmation
if needs_decryption and not password_required and not args.yes:
    # Key file was used, so ask for confirmation
    if input(Fore.CYAN + "Do you want to proceed? (y/n): ").lower() not in ["y", "yes"]:
        print(Fore.GREEN + "Cancel [{}]".format(" ".join(sys.argv)))
        print(Fore.GREEN + "See you soon. :)")
        sys.exit(0)
elif needs_decryption and not password_required and args.yes:
    print(Fore.GREEN + "Auto-yes mode enabled - proceeding with selected options...")
    print("")

# Check server health before proceeding (retry up to 50 times with 1 second interval)
print(Fore.YELLOW + "Checking server health...")
health_check_url = f"http://{TUMBLEBUG_SERVER}/tumblebug/readyz"
max_retries = 50
retry_interval = 1  # seconds

for attempt in range(1, max_retries + 1):
    try:
        health_response = requests.get(health_check_url, headers=HEADERS, timeout=5)
        if health_response.status_code == 200:
            print(Fore.GREEN + f"Tumblebug Server is healthy. (attempt {attempt}/{max_retries})\n")
            break
        else:
            if attempt < max_retries:
                print(Fore.YELLOW + f"Health check failed (status {health_response.status_code}), retrying... ({attempt}/{max_retries})")
                time.sleep(retry_interval)
            else:
                print(Fore.RED + f"Tumblebug health check failed with status {health_response.status_code} after {max_retries} attempts.")
                sys.exit(1)
    except requests.exceptions.RequestException as e:
        if attempt < max_retries:
            print(Fore.YELLOW + f"Connection failed, retrying... ({attempt}/{max_retries})")
            time.sleep(retry_interval)
        else:
            print(Fore.RED + f"Failed to connect to server after {max_retries} attempts. Check the server address and try again.")
            sys.exit(1)


# Function to encrypt credentials using AES and RSA public key
def encrypt_credential_value_with_publickey(public_key_pem, credentials):
    public_key = RSA.import_key(public_key_pem)
    rsa_cipher = PKCS1_OAEP.new(public_key, hashAlgo=SHA256)
    aes_key = get_random_bytes(32)  # AES-256 key

    encrypted_credentials = {}
    for k, v in credentials.items():
        # Encrypt using AES
        aes_cipher = AES.new(aes_key, AES.MODE_CBC)
        ciphertext = aes_cipher.encrypt(pad(v.encode(), AES.block_size))
        encrypted_credentials[k] = base64.b64encode(aes_cipher.iv + ciphertext).decode()

    # Encrypt AES key with RSA and encode in Base64
    encrypted_aes_key = base64.b64encode(rsa_cipher.encrypt(aes_key)).decode()

    # Clear AES key from memory
    del aes_key

    return encrypted_credentials, encrypted_aes_key


def register_credential(holder_name, provider, credentials):
    try:
        if all(credentials.values()):
            # Step 1: Get the public key for encryption
            public_key_response = requests.get(f"http://{TUMBLEBUG_SERVER}/tumblebug/credential/publicKey", headers=HEADERS)
            if public_key_response.status_code != 200:
                return holder_name, provider, "Failed to retrieve public key, Skip", Fore.RED

            public_key_data = public_key_response.json()
            public_key = public_key_data["publicKey"]
            public_key_token_id = public_key_data["publicKeyTokenId"]

            # Step 2: Encrypt the credentials using AES and RSA public key
            encrypted_credentials, encrypted_aes_key = encrypt_credential_value_with_publickey(public_key, credentials)

            # Step 3: Prepare the payload with the encrypted credentials and AES key
            credential_payload = {
                "credentialHolder": holder_name,
                "credentialKeyValueList": [{"key": k, "value": v} for k, v in encrypted_credentials.items()],
                "providerName": provider,
                "publicKeyTokenId": public_key_token_id,
                "encryptedClientAesKeyByPublicKey": encrypted_aes_key,
            }

            holder_label = f"[{holder_name}] " if holder_name != "admin" else ""
            print(Fore.YELLOW + f"- {holder_label}Registering and validating credentials for {provider.upper()}...")
            # Step 4: Register the encrypted credentials
            response = requests.post(f"http://{TUMBLEBUG_SERVER}/tumblebug/credential", json=credential_payload, headers=HEADERS)

            if response.status_code == 200:
                # Extract relevant data for message
                result_data = response.json()
                message = print_credential_info(result_data)
                return holder_name, provider, message, Fore.GREEN
            else:
                message = response.json().get("message", response.text)
                return holder_name, provider, message, Fore.RED
        else:
            message = "Incomplete credential data, Skip"
            return holder_name, provider, message, Fore.RED
    except Exception as e:
        message = "Error registering credentials: " + str(e)
        return holder_name, provider, message, Fore.RED


# Function to print formatted credential information
def print_credential_info(response):
    if "credentialName" in response and "credentialHolder" in response:
        # Print credential name and holder in bold
        print(Fore.YELLOW + f"\n{response['credentialName'].upper()} (holder: {response['credentialHolder']})" + Style.RESET_ALL)

    if "allConnections" in response and "connectionconfig" in response["allConnections"]:
        # Print the explanation line with icons
        print(
            Style.BRIGHT
            + "Registered Connections: "
            + Style.RESET_ALL
            + Fore.GREEN
            + "✓"
            + Style.RESET_ALL
            + "=verified, "
            + Fore.RED
            + "✗"
            + Style.RESET_ALL
            + "=unverified, "
            + Fore.MAGENTA
            + "*"
            + Style.RESET_ALL
            + "=representative zone"
        )

        # Group connections by region, collecting all zones and tracking verification status
        region_groups = {}
        for conn in response["allConnections"]["connectionconfig"]:
            if conn["providerName"] == response["providerName"]:
                region = conn["regionZoneInfo"]["assignedRegion"]
                zone = conn["regionZoneInfo"]["assignedZone"]
                config_name = conn["configName"]
                is_verified = conn["verified"]
                is_representative = conn["regionRepresentative"]

                if region not in region_groups:
                    region_groups[region] = {
                        "base_config_name": config_name,  # Use first (shortest) config name as base
                        "zones": set(),  # Use set for O(1) lookups
                        "all_verified": True,  # Track if all connections are verified
                        "any_verified": False,  # Track if any connection is verified
                    }

                # Update base_config_name to use shortest name (usually the base without zone suffix)
                if len(config_name) < len(region_groups[region]["base_config_name"]):
                    region_groups[region]["base_config_name"] = config_name

                # Track verification status across all connections in this region
                region_groups[region]["all_verified"] = region_groups[region]["all_verified"] and is_verified
                region_groups[region]["any_verified"] = region_groups[region]["any_verified"] or is_verified

                # Add zone info - if zone is representative in any connection, mark it as representative
                existing_zone = None
                for z in region_groups[region]["zones"]:
                    if z[0] == zone:
                        existing_zone = z
                        break

                if existing_zone:
                    # If this zone is representative, update the existing entry
                    if is_representative and not existing_zone[1]:
                        region_groups[region]["zones"].discard(existing_zone)
                        region_groups[region]["zones"].add((zone, True))
                else:
                    region_groups[region]["zones"].add((zone, is_representative))

        # Print in format: icon base-config-name : region (zone1* zone2 zone3)
        for region, data in sorted(region_groups.items()):
            # Format zones: representative zones with asterisk, sorted with representative first
            zone_list = sorted(data["zones"], key=lambda x: (not x[1], x[0]))  # Representative first, then alphabetical
            zone_displays = []
            for zone, is_rep in zone_list:
                if is_rep:
                    zone_displays.append(f"{Fore.MAGENTA}{zone}*{Style.RESET_ALL}")
                else:
                    zone_displays.append(zone)

            zones_str = " ".join(zone_displays)

            # Use green check if all verified, red X if none verified, yellow ~ if mixed
            config_name = data["base_config_name"]
            if data["all_verified"]:
                print(f"{Fore.GREEN}✓{Style.RESET_ALL} {Fore.CYAN}{config_name}{Style.RESET_ALL} : {Fore.YELLOW}{region}{Style.RESET_ALL} ({zones_str})")
            elif not data["any_verified"]:
                print(f"{Fore.RED}✗{Style.RESET_ALL} {Fore.CYAN}{config_name}{Style.RESET_ALL} : {Fore.YELLOW}{region}{Style.RESET_ALL} ({zones_str})")
            else:
                # Mixed verification status
                print(f"{Fore.YELLOW}~{Style.RESET_ALL} {Fore.CYAN}{config_name}{Style.RESET_ALL} : {Fore.YELLOW}{region}{Style.RESET_ALL} ({zones_str})")


# Function to fetch price information from CSPs
def fetch_price():
    try:
        # FetchPrice API is a POST endpoint with no required body
        response = requests.post(f"http://{TUMBLEBUG_SERVER}/tumblebug/fetchPrice", headers=HEADERS)
        response.raise_for_status()  # Will raise an exception for HTTP error codes

        response_json = response.json()
        if response_json is None:  # Check if response.json() returned None
            response_json = {"error": "No content returned"}

        # Log success message
        print(f"Price fetching initiated: {response_json.get('message', 'No message returned')}")
        return response_json
    except requests.RequestException as e:
        error_msg = f"Failed to fetch prices: {str(e)}"
        print(Fore.RED + error_msg)
        return {"error": error_msg}


# Register credentials to Tumblebug if requested
if run_credentials:
    # all_holders was already parsed before health check to ensure all user inputs complete first
    holder_names = list(all_holders.keys())
    print(Fore.YELLOW + f"\nRegistering credentials for {len(holder_names)} credential holder(s): {', '.join(holder_names)}...")

    # Validate holder names (only lowercase alphanumeric and underscores allowed; no hyphens)
    import re
    holder_name_pattern = re.compile(r'^[a-z0-9_]+$')
    for holder_name in holder_names:
        if not holder_name_pattern.match(holder_name.lower()):
            print(Fore.RED + f"\nError: Invalid credential holder name '{holder_name}'.")
            print(Fore.RED + "  Holder names must contain only lowercase alphanumeric characters and underscores [a-z0-9_].")
            print(Fore.RED + "  Hyphens (-) are not allowed (reserved as connection name delimiters).")
            print(Fore.RED + f"  Suggestion: Use '{holder_name.lower().replace('-', '_')}' instead.")
            sys.exit(1)

    # Register credentials for all holders and their providers
    for holder_name in holder_names:
        cred_data = all_holders[holder_name]
        if not cred_data:
            print(Fore.YELLOW + f"\n  Skipping holder '{holder_name}' (no credentials defined)")
            continue

        if holder_name != "admin":
            print(Fore.CYAN + f"\n  === Credential Holder: {holder_name} ===")

        # Register credentials to TumblebugServer using ThreadPoolExecutor
        with ThreadPoolExecutor(max_workers=20) as executor:
            future_to_key = {
                executor.submit(register_credential, holder_name, provider, credentials): (holder_name, provider)
                for provider, credentials in cred_data.items()
            }
            for future in as_completed(future_to_key):
                holder, provider, message, color = future.result()
                if message is None:
                    message = ""  # Handle NoneType message
                else:
                    print("")
                    holder_label = f"[{holder}] " if holder != "admin" else ""
                    print(color + f"- {holder_label}{provider.upper()}: {message}")
                    # Only call print_credential_info if registration was successful (message is a dict)
                    if color == Fore.GREEN and isinstance(message, dict):
                        print_credential_info(message)

# Load assets (specs and images) if requested
if run_load_assets:
    # use_backup was already determined earlier (before confirmation prompt)

    if use_backup:
        # Restore from backup
        print(Fore.YELLOW + "\n📦 Restoring database from backup...")
        print(Fore.RESET)

        try:
            # Run restore script
            restore_script = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "scripts", "restore-assets.sh")

            if not os.path.isfile(restore_script):
                print(Fore.RED + f"Error: Restore script not found at {restore_script}")
                print(Fore.YELLOW + "Falling back to standard initialization...")
                use_backup = False
            else:
                # Run restore script with the backup file
                # Note: Script permissions are managed by Git (should be executable)
                result = subprocess.run(
                    [restore_script, backup_db_path],
                    env={**os.environ, "RESTORE_SKIP_CONFIRM": "yes"},  # Skip confirmation in script
                    capture_output=True,
                    text=True,
                    timeout=600,  # 10 minutes timeout to prevent hanging
                )

                if result.returncode == 0:
                    print(Fore.GREEN + "✅ Database restored successfully from backup!")
                    print(Fore.CYAN + "   Initialization time: ~1 minute (instead of ~20 minutes)")
                    print(Fore.CYAN + "   Restored: Specs, Images, and Pricing data")

                    # Create 'system' namespace if it doesn't exist (required for image/spec operations)
                    print(Fore.YELLOW + "\n   Ensuring 'system' namespace exists...")
                    try:
                        ns_check_response = requests.get(f"http://{TUMBLEBUG_SERVER}/tumblebug/ns/system", headers=HEADERS)
                        if ns_check_response.status_code == 404:
                            # Create system namespace
                            ns_payload = {"name": "system", "description": "Namespace for common resources"}
                            ns_create_response = requests.post(f"http://{TUMBLEBUG_SERVER}/tumblebug/ns", json=ns_payload, headers=HEADERS)
                            if ns_create_response.status_code == 200:
                                print(Fore.GREEN + "   ✅ 'system' namespace created")
                            else:
                                print(Fore.YELLOW + f"   ⚠️  Failed to create 'system' namespace: {ns_create_response.text}")
                        else:
                            print(Fore.GREEN + "   ✅ 'system' namespace already exists")
                    except Exception as ns_err:
                        print(Fore.YELLOW + f"   ⚠️  Namespace check failed: {str(ns_err)}")

                    # Skip the load_resources call since DB is already populated
                    run_load_assets = False  # Mark as completed
                    # Also skip fetch_price since pricing data is included in backup
                    if run_fetch_price:
                        print(Fore.GREEN + "   ℹ️  Skipping price fetch - pricing data already included in backup")
                        run_fetch_price = False
                else:
                    print(Fore.RED + f"❌ Database restore failed: {result.stderr}")
                    print(Fore.YELLOW + "Falling back to standard initialization...")
                    use_backup = False
        except Exception as e:
            print(Fore.RED + f"❌ Error during database restore: {str(e)}")
            print(Fore.YELLOW + "Falling back to standard initialization...")
            use_backup = False

    # If not using backup or backup failed, proceed with standard initialization
    if not use_backup and run_load_assets:
        # Adjust estimated time based on Azure inclusion
        if include_azure:
            expected_completion_time_seconds = 2400  # 40 minutes for Azure
            print(Fore.YELLOW + "\nLoading common Specs and Images from ALL CSPs including Azure...")
            print(Fore.MAGENTA + "⚠️  This may take 40+ minutes due to Azure image fetch")
        else:
            print(Fore.YELLOW + "\nLoading common Specs and Images from CSPs (excluding Azure)...")
            print(Fore.CYAN + f"Estimated time: ~{expected_completion_time_seconds}s")
        print(Fore.RESET)

    # Function to perform the HTTP request and handle exceptions
    def load_resources():
        global response_json
        try:
            # Build URL with includeAzure parameter
            url = f"http://{TUMBLEBUG_SERVER}/tumblebug/loadAssets"
            if include_azure:
                url += "?includeAzure=true"

            response = requests.get(url, headers=HEADERS)
            response.raise_for_status()  # Will raise an exception for HTTP error codes
            response_json = response.json()
        except requests.RequestException as e:
            response_json = {"error": str(e)}
        finally:
            event.set()  # Signal that the request is complete regardless of success or failure

    # Only run standard initialization if we didn't use backup
    if not use_backup and run_load_assets:
        # Start time
        start_time = time.time()

        # Event object to signal the request completion
        event = threading.Event()

        # Start the network request in a separate thread
        thread = threading.Thread(target=load_resources)
        thread.start()

        # Use a simple spinner instead of progress bar for indeterminate duration
        # This avoids the stuttering issue caused by inaccurate time.sleep() and
        # provides smoother visual feedback for operations with unpredictable duration
        spinner_chars = ["⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"]
        spinner_idx = 0
        elapsed = 0
        last_print_sec = -1  # Track last printed second for non-TTY mode
        is_tty = sys.stdout.isatty()  # Detect if running in interactive terminal

        while not event.is_set():
            # Use shorter sleep for smoother animation
            time.sleep(0.08)
            elapsed += 0.08
            spinner_idx = (spinner_idx + 1) % len(spinner_chars)

            # Format elapsed time
            mins, secs = divmod(int(elapsed), 60)
            time_str = f"{mins}m {secs}s" if mins > 0 else f"{secs}s"

            if is_tty:
                # Interactive terminal: spinner animation with carriage return
                print(
                    f"\r{spinner_chars[spinner_idx]} Loading... {time_str} (expected: ~{expected_completion_time_seconds}s)",
                    end="",
                    flush=True,
                )
            else:
                # Non-interactive (SSH, pipe, redirect): print only once per second
                current_sec = int(elapsed)
                if current_sec > last_print_sec:
                    last_print_sec = current_sec
                    # Print simple progress every 30 seconds to avoid log spam
                    if current_sec % 30 == 0:
                        print(f"Loading... {time_str}", flush=True)

        # Clear the spinner line (only for TTY)
        if is_tty:
            print(f"\r{' ' * 80}\r", end="")

        # Wait for the thread to complete
        thread.join()

        # Calculate duration
        end_time = time.time()
        duration = end_time - start_time

        # Handling output based on the API response
        if "error" in response_json:
            print(Fore.RED + "Error during resource loading: " + response_json["error"])
            exit(1)
        elif response_json:
            print(Fore.CYAN + f"\nLoading completed (elapsed: {int(duration)}s)")
        else:
            print(Fore.RED + "No data returned from the API.")

# Fetch price information if requested
if run_fetch_price:
    # Print a message for initiating price fetching and say that this final operation can be run in the background
    print(Fore.CYAN + "\nInitiating price fetching information from all CSPs...")
    print(Fore.CYAN + "Price for Specs will be updated (it may take around 10 mins).")
    print(Fore.YELLOW + "\nYou can run this procedure in the background using ctrl+c or ctrl+z.")
    # Start the price fetching
    fetch_price()

# Load templates if requested
if run_load_templates:
    import glob
    import json as json_module

    templates_dir = os.path.join(os.path.dirname(os.path.abspath(__file__)), "templates")

    if os.path.isdir(templates_dir):
        template_files = sorted(glob.glob(os.path.join(templates_dir, "*.json")))
        if template_files:
            print(Fore.YELLOW + f"\nLoading {len(template_files)} template(s) from {templates_dir}...")

            for tf in template_files:
                try:
                    with open(tf, "r") as f:
                        template_data = json_module.load(f)

                    # Determine namespace (default to 'system' for global templates)
                    ns_id = template_data.pop("nsId", "system")

                    # Determine template type from resourceType field or content-based auto-detection.
                    # Detection priority:
                    # 1. 'resourceType' field (e.g., "mci", "vNet")
                    #    - Consistent with Go model's ResourceType field
                    #    - Works for both hand-crafted files and GET API response saved as file
                    # 2. Content-based detection (presence of 'mciDynamicReq' or 'vNetReq' key)
                    resource_type = template_data.pop("resourceType", None)
                    if resource_type == "mci":
                        template_type = "mci"
                    elif resource_type == "vNet":
                        template_type = "vNet"
                    elif resource_type == "securityGroup":
                        template_type = "securityGroup"
                    elif "mciDynamicReq" in template_data:
                        template_type = "mci"
                    elif "vNetReq" in template_data or "vNetPolicy" in template_data:
                        template_type = "vNet"
                    elif "securityGroupReq" in template_data:
                        template_type = "securityGroup"
                    else:
                        print(
                            Fore.RED + f"  ❌ Cannot detect template type for {os.path.basename(tf)}: "
                            f"no 'resourceType' or known request body key found. Skipping."
                        )
                        continue

                    # Remove fields that are in the Info model but not in the Req model,
                    # in case the file is a saved GET API response
                    for extra_field in ["id", "uid", "source", "createdAt", "updatedAt", "systemLabel"]:
                        template_data.pop(extra_field, None)

                    # Ensure namespace exists
                    try:
                        ns_check = requests.get(f"http://{TUMBLEBUG_SERVER}/tumblebug/ns/{ns_id}", headers=HEADERS, timeout=10)
                        if ns_check.status_code == 404 or ns_check.status_code == 400:
                            ns_payload = {"name": ns_id, "description": f"Namespace for templates"}
                            requests.post(f"http://{TUMBLEBUG_SERVER}/tumblebug/ns", json=ns_payload, headers=HEADERS, timeout=10)
                    except Exception:
                        pass

                    # POST template to appropriate API endpoint based on template type
                    url = f"http://{TUMBLEBUG_SERVER}/tumblebug/ns/{ns_id}/template/{template_type}"
                    resp = requests.post(url, json=template_data, headers=HEADERS, timeout=30)

                    template_name = template_data.get("name", os.path.basename(tf))
                    if resp.status_code == 200:
                        print(Fore.GREEN + f"  ✅ Template loaded: {template_name} (type: {template_type}, ns: {ns_id})")
                    elif "already exists" in resp.text:
                        print(Fore.CYAN + f"  ℹ️  Template already exists: {template_name} (type: {template_type}, ns: {ns_id})")
                    else:
                        print(Fore.RED + f"  ❌ Failed to load template {template_name}: {resp.text}")
                except Exception as e:
                    print(Fore.RED + f"  ❌ Error loading {os.path.basename(tf)}: {str(e)}")
        else:
            print(Fore.CYAN + f"\nNo template files found in {templates_dir}")
    else:
        print(Fore.CYAN + f"\nTemplates directory not found: {templates_dir}")

# Final message and set initialization status
if run_all or (run_credentials and run_load_assets):
    # Notify CB-Tumblebug that initialization is complete
    try:
        init_url = f"http://{TUMBLEBUG_SERVER}/tumblebug/readyz/init"
        response = requests.put(init_url, headers=HEADERS, timeout=10)
        if response.status_code == 200:
            print(Fore.GREEN + "\n[OK] System initialization status has been set.")
        else:
            print(Fore.YELLOW + f"\n[Warning] Failed to set initialization status: {response.status_code}")
    except Exception as e:
        print(Fore.YELLOW + f"\n[Warning] Could not notify initialization completion: {e}")

    print(Fore.YELLOW + f"\nThe system is ready to use.")
