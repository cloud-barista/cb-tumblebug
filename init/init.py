#!/usr/bin/env python3

import os
import base64
import time
import sys
import argparse
import threading
from concurrent.futures import ThreadPoolExecutor, as_completed
import subprocess
import requests
import yaml
from alive_progress import alive_bar
from tabulate import tabulate
from colorama import Fore, Style, init
from getpass import getpass
from Crypto.PublicKey import RSA
from Crypto.Hash import SHA256
from Crypto.Cipher import PKCS1_OAEP
from Crypto.Cipher import AES
from Crypto.Random import get_random_bytes
from Crypto.Util.Padding import pad

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
  %(prog)s --credentials --load-assets        # Register credentials and load assets
  %(prog)s -y --credentials --fetch-price     # Register credentials and fetch price (no confirmation)
    """
)
parser.add_argument('-y', '--yes', action='store_true',
                    help='Automatically answer yes to prompts and proceed without confirmation')
parser.add_argument('--credentials', '--credentials-only', action='store_true', dest='credentials_only',
                    help='Register cloud credentials only')
parser.add_argument('--load-assets', '--load-assets-only', action='store_true', dest='load_assets_only',
                    help='Load common specs and images only')
parser.add_argument('--fetch-price', '--fetch-price-only', action='store_true', dest='fetch_price_only',
                    help='Fetch price information only')
args = parser.parse_args()

# Determine which operations to run
# If no specific options are provided, run all operations (default behavior)
run_all = not (args.credentials_only or args.load_assets_only or args.fetch_price_only)
run_credentials = run_all or args.credentials_only
run_load_assets = run_all or args.load_assets_only
run_fetch_price = run_all or args.fetch_price_only

# Initialize colorama
init(autoreset=True)

# Configuration
TUMBLEBUG_SERVER = os.getenv('TUMBLEBUG_SERVER', 'localhost:1323')
TB_API_USERNAME = os.getenv('TB_API_USERNAME', 'default')
TB_API_PASSWORD = os.getenv('TB_API_PASSWORD', 'default')
AUTH = f"Basic {base64.b64encode(f'{TB_API_USERNAME}:{TB_API_PASSWORD}'.encode()).decode()}"
HEADERS = {'Authorization': AUTH, 'Content-Type': 'application/json'}

CRED_FILE_NAME_ENC = "credentials.yaml.enc"
CRED_PATH = os.path.join(os.path.expanduser('~'), '.cloud-barista')
ENC_FILE_PATH = os.path.join(CRED_PATH, CRED_FILE_NAME_ENC)
KEY_FILE = os.path.join(CRED_PATH, ".tmp_enc_key")

expected_completion_time_seconds = 180

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
            ['openssl', 'enc', '-aes-256-cbc', '-d', '-pbkdf2', '-in', enc_file_path, '-pass', f'pass:{key}'],
            check=True,
            capture_output=True
        )
        if result.returncode != 0:
            return None, "Decryption failed."
        return result.stdout.decode('utf-8'), None
    except subprocess.CalledProcessError as e:
        return None, f"Decryption error: {e.stderr.decode('utf-8')}"

def get_decryption_key():
    # Try using the key from the key file
    if os.path.isfile(KEY_FILE):
        with open(KEY_FILE, 'r') as kf:
            key = kf.read().strip()
            print(Fore.YELLOW + f"Using key from {KEY_FILE} to decrypt the credentials file.")
            decrypted_content, error = decrypt_credentials(ENC_FILE_PATH, key)
            if error is None:
                return decrypted_content
            print(Fore.RED + error)

    # Prompt for password up to 3 times if the key file is not used or fails
    for attempt in range(3):
        password = getpass(f"Enter the password to decrypt the credentials file (attempt {attempt + 1}/3): ")
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

# Check server health before proceeding
print(Fore.YELLOW + "Checking server health...")
health_check_url = f"http://{TUMBLEBUG_SERVER}/tumblebug/readyz"
try:
    health_response = requests.get(health_check_url, headers=HEADERS)
    if health_response.status_code == 200:
        print(Fore.GREEN + "Tumblebug Server is healthy.\n")
    else:
        print(Fore.RED + f"Tumblebug health check failed with status {health_response.status_code}.")
        sys.exit(1)
except requests.exceptions.RequestException as e:
    print(Fore.RED + f"Failed to connect to server. Check the server address and try again.")
    sys.exit(1)

# Check for database backup availability early (before asking for confirmation)
backup_db_path = os.path.join(os.path.dirname(os.path.abspath(__file__)), '..', 'assets', 'assets.dump.gz')
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
            ['git', 'log', '-1', '--format=%ct', '--', 'assets/assets.dump.gz'],
            cwd=os.path.dirname(os.path.abspath(__file__)) + '/..',
            capture_output=True,
            text=True,
            timeout=5,
            check=False  # Don't raise exception on non-zero exit
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
    print(Fore.CYAN + "="*80)
    print(Fore.CYAN + "🚀 Choose Initialization Method")
    print(Fore.CYAN + "="*80)
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
        if choice in ['a']:
            use_backup = True
            include_azure = False
            break
        elif choice in ['b']:
            use_backup = False
            include_azure = False
            break
        elif choice in ['c']:
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
        if choice in ['y', 'yes']:
            include_azure = True
            break
        elif choice in ['n', 'no', '']:
            include_azure = False
            break
        else:
            print(Fore.RED + "Invalid input. Please enter 'y' or 'n'.")
    print("")

# Display what will be executed (after user choice)
operations = []
if run_credentials:
    operations.append("Register credentials")
if run_load_assets:
    if use_backup:
        operations.append(f"Load assets from backup ({backup_size_mb:.1f} MB - includes specs, images, pricing)")
    else:
        operations.append("Load assets (fetch from CSPs)")
if run_fetch_price and not use_backup:
    operations.append("Fetch price information")

print(Fore.YELLOW + "\n" + "="*80)
print(Fore.YELLOW + "Operations to be performed:")
print(Fore.YELLOW + "="*80)
for i, op in enumerate(operations, 1):
    print(Fore.CYAN + f"  {i}. {op}")

if use_backup:
    print("")
    print(Fore.GREEN + "  ℹ️  Using backup - Steps 2 & 3 completed in ~1 minute")
elif run_load_assets and not backup_available:
    print("")
    print(Fore.YELLOW + "  ⚠️  No backup found - will fetch from CSPs (~20 minutes)")

print(Fore.YELLOW + "="*80)
print("")

# Wait for user input to proceed
if not args.yes:
    if input(Fore.CYAN + 'Do you want to proceed? (y/n): ').lower() not in ['y', 'yes']:
        print(Fore.GREEN + "Cancel [{}]".format(' '.join(sys.argv)))
        print(Fore.GREEN + "See you soon. :)")
        sys.exit(0)
else:
    print(Fore.GREEN + "Auto-yes mode enabled - proceeding with selected options...")
    print("")

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

def register_credential(provider, credentials):
    try:
        if all(credentials.values()):
            # Step 1: Get the public key for encryption
            public_key_response = requests.get(f"http://{TUMBLEBUG_SERVER}/tumblebug/credential/publicKey", headers=HEADERS)
            if public_key_response.status_code != 200:
                return provider, "Failed to retrieve public key, Skip", Fore.RED

            public_key_data = public_key_response.json()
            public_key = public_key_data['publicKey']
            public_key_token_id = public_key_data['publicKeyTokenId']

            # Step 2: Encrypt the credentials using AES and RSA public key
            encrypted_credentials, encrypted_aes_key = encrypt_credential_value_with_publickey(public_key, credentials)

            # Step 3: Prepare the payload with the encrypted credentials and AES key
            credential_payload = {
                "credentialHolder": "admin",
                "credentialKeyValueList": [{"key": k, "value": v} for k, v in encrypted_credentials.items()],
                "providerName": provider,
                "publicKeyTokenId": public_key_token_id,
                "encryptedClientAesKeyByPublicKey": encrypted_aes_key
            }

            print(Fore.YELLOW + f"- Registering and validating credentials for {provider.upper()}...")
            # Step 4: Register the encrypted credentials
            response = requests.post(f"http://{TUMBLEBUG_SERVER}/tumblebug/credential", json=credential_payload, headers=HEADERS)

            if response.status_code == 200:
                # Extract relevant data for message
                result_data = response.json()
                message = print_credential_info(result_data)
                return provider, message, Fore.GREEN
            else:
                message = response.json().get('message', response.text)
                return provider, message, Fore.RED
        else:
            message = "Incomplete credential data, Skip"
            return provider, message, Fore.RED
    except Exception as e:
        message = "Error registering credentials: " + str(e)
        return provider, message, Fore.RED


# Function to print formatted credential information
def print_credential_info(response):
    if 'credentialName' in response and 'credentialHolder' in response:
        # Print credential name and holder in bold
        print(Fore.YELLOW + f"\n{response['credentialName'].upper()} (holder: {response['credentialHolder']})" + Style.RESET_ALL)
        
    if 'allConnections' in response and 'connectionconfig' in response['allConnections']:
        # Print the explanation line in yellow
        print(Style.BRIGHT + "Registered Connections" + Fore.GREEN + " [verified]" + Fore.MAGENTA + "[region representative]" + Style.RESET_ALL)

        # Prepare table headers and rows
        headers = ["Config Name", "Assigned Region", "Assigned Zone"]
        table_rows = []
        for conn in response['allConnections']['connectionconfig']:
            if conn['providerName'] == response['providerName']:
                # Config name with green color if verified
                config_name_display = Fore.GREEN + conn['configName'] + Style.RESET_ALL if conn['verified'] else conn['configName']

                # Assigned Zone with pink color if region representative
                assigned_zone_display = Fore.MAGENTA + conn['regionZoneInfo']['assignedZone'] + Style.RESET_ALL if conn['regionRepresentative'] else conn['regionZoneInfo']['assignedZone']

                # Add row to the table
                table_rows.append([
                    config_name_display,
                    conn['regionZoneInfo']['assignedRegion'],
                    assigned_zone_display
                ])

        # Print table
        print(tabulate(table_rows, headers, tablefmt="grid"))

# Function to fetch price information from CSPs
def fetch_price():
    try:
        # FetchPrice API is a POST endpoint with no required body
        response = requests.post(f"http://{TUMBLEBUG_SERVER}/tumblebug/fetchPrice", headers=HEADERS)
        response.raise_for_status()  # Will raise an exception for HTTP error codes
        
        response_json = response.json()
        if response_json is None:  # Check if response.json() returned None
            response_json = {'error': 'No content returned'}
        
        # Log success message
        print(f"Price fetching initiated: {response_json.get('message', 'No message returned')}")
        return response_json
    except requests.RequestException as e:
        error_msg = f'Failed to fetch prices: {str(e)}'
        print(Fore.RED + error_msg)
        return {'error': error_msg}


# Register credentials if requested
if run_credentials:
    # Get the decryption key and decrypt the credentials file
    decrypted_content = get_decryption_key()
    cred_data = yaml.safe_load(decrypted_content)['credentialholder']['admin']
    
    print(Fore.YELLOW + f"\nRegistering all valid credentials for all cloud regions...")

    # Register credentials to TumblebugServer using ThreadPoolExecutor
    with ThreadPoolExecutor(max_workers=20) as executor:
        future_to_provider = {executor.submit(register_credential, provider, credentials): provider for provider, credentials in cred_data.items()}
        for future in as_completed(future_to_provider):
            provider, message, color = future.result()
            if message is None:
                message = ""  # Handle NoneType message
            else:
                print("")
                print(color + f"- {provider.upper()}: {message}")
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
            restore_script = os.path.join(os.path.dirname(os.path.abspath(__file__)), '..', 'scripts', 'restore-assets.sh')
            
            if not os.path.isfile(restore_script):
                print(Fore.RED + f"Error: Restore script not found at {restore_script}")
                print(Fore.YELLOW + "Falling back to standard initialization...")
                use_backup = False
            else:
                # Run restore script with the backup file
                # Note: Script permissions are managed by Git (should be executable)
                result = subprocess.run(
                    [restore_script, backup_db_path],
                    env={**os.environ, 'RESTORE_SKIP_CONFIRM': 'yes'},  # Skip confirmation in script
                    capture_output=True,
                    text=True,
                    timeout=600  # 10 minutes timeout to prevent hanging
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
            response_json = {'error': str(e)}
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

        # Expected duration and progress bar
        update_interval = 0.1  # Update interval in seconds
        step_multiplier = 10  # Increase this to make the bar move faster visually
        total_steps = expected_completion_time_seconds * step_multiplier

        # Progress bar with 'smooth' style and manual updates enabled
        with alive_bar(total_steps, bar="smooth", manual=True, stats=False, elapsed=False) as bar:
            elapsed_steps = 0  # Track the number of elapsed steps
            while not event.is_set():  # Continue until the event signals completion
                time.sleep(update_interval)  # Wait for the specified update interval
                elapsed_steps += 1  # Increment the step count

                # Update the bar text with elapsed and expected time
                # bar.text = f"Expected: {expected_completion_time_seconds}s"
                bar(elapsed_steps / total_steps)  # Update the progress bar manually

            # Ensure the bar reaches 100% when the task completes
            bar(1.0)
        # Wait for the thread to complete
        thread.join()

        # Calculate duration
        end_time = time.time()
        duration = end_time - start_time

        # Handling output based on the API response
        if 'error' in response_json:
            print(Fore.RED + "Error during resource loading: " + response_json['error'])
            exit(1)
        elif response_json:
            print(Fore.CYAN + f"\nLoading completed (elapsed: {duration}s)")
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

# Final message
if run_all or (run_credentials and run_load_assets):
    print(Fore.YELLOW + f"\nThe system is ready to use.")
