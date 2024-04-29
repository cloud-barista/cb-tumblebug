#!/usr/bin/env python3

import requests
import yaml
import os
import base64
import time
import sys
from tqdm import tqdm
import threading
from concurrent.futures import ThreadPoolExecutor, as_completed
from colorama import Fore, Style, init

# Initialize colorama
init(autoreset=True)

# Configuration
TUMBLEBUG_SERVER = 'localhost:1323'
API_USERNAME = 'default'
API_PASSWORD = 'default'
AUTH = f"Basic {base64.b64encode(f'{API_USERNAME}:{API_PASSWORD}'.encode()).decode()}"
HEADERS = {'Authorization': AUTH, 'Content-Type': 'application/json'}

CBTUMBLEBUG_ROOT = os.getenv('CBTUMBLEBUG_ROOT', os.path.abspath(os.path.join(os.path.dirname(__file__), '..')))
CRED_FILE_NAME = "credentials.yaml"
CRED_PATH = os.path.join(os.path.expanduser('~'), '.cloud-barista')

expected_duration = 240

# Check for credential path
if not os.path.exists(CRED_PATH):
    print(Fore.RED + "Error: CRED_PATH does not exist. Please run scripts/genCredential.sh first.")
    sys.exit(1)
elif not os.path.isfile(os.path.join(CRED_PATH, CRED_FILE_NAME)):
    print(Fore.RED + "Error: CRED_FILE_NAME does not exist. Please check if it has been generated.")
    sys.exit(1)

# Wait for user input to proceed
print(Fore.YELLOW + "\nRegistering credentials and Loading common Specs and Images takes time")
if input(Fore.CYAN + 'Do you want to proceed ? (y/n) : ').lower() not in ['y', 'yes']:
    print(Fore.GREEN + "Cancel [{}]".format(' '.join(sys.argv)))
    print(Fore.GREEN + "See you soon. :)")
    sys.exit(0)

# Load credentials from YAML file
with open(os.path.join(CRED_PATH, CRED_FILE_NAME), 'r') as file:
    cred_data = yaml.safe_load(file)['credentialholder']['admin']



print(Fore.CYAN + f"\n[Registering all valid credentials for all cloud regions]")

# Function to register credentials
def register_credential(provider, credentials):
    try:
        if all(credentials.values()):
            credential_payload = {
                "credentialHolder": "admin",
                "keyValueInfoList": [{"key": k, "value": v} for k, v in credentials.items()],
                "providerName": provider
            }
            response = requests.post(f"http://{TUMBLEBUG_SERVER}/tumblebug/credential", json=credential_payload, headers=HEADERS)
            return provider, response.json(), Fore.GREEN
        else:
            return provider, "Incomplete credential data, Skip", Fore.YELLOW
    except Exception as e:
        return provider, f"Error registering credentials: {str(e)}", Fore.RED

# Register credentials to TumblebugServer using ThreadPoolExecutor
with ThreadPoolExecutor(max_workers=5) as executor:
    future_to_provider = {executor.submit(register_credential, provider, credentials): provider for provider, credentials in cred_data.items()}
    for future in as_completed(future_to_provider):
        provider, response, color = future.result()
        print(color + f"- {provider}: {response}")

print(Fore.CYAN + f"\n[Loading common Specs and Images]")
print(Fore.RESET)

# Function to perform the HTTP request and handle exceptions
def load_resources():
    global response_json
    try:
        response = requests.get(f"http://{TUMBLEBUG_SERVER}/tumblebug/loadCommonResource", headers=HEADERS)
        response.raise_for_status()  # Will raise an exception for HTTP error codes
        response_json = response.json()
    except requests.RequestException as e:
        response_json = {'error': str(e)}
    finally:
        event.set()  # Signal that the request is complete regardless of success or failure

# Start time
start_time = time.time()

# Event object to signal the request completion
event = threading.Event()

# Start the network request in a separate thread
thread = threading.Thread(target=load_resources)
thread.start()

# Expected duration and progress bar

with tqdm(total=expected_duration, desc="Progress", unit='s') as pbar:
    while not event.is_set():
        time.sleep(1)
        pbar.update(1)
    pbar.update(expected_duration - pbar.n)  # Ensure the progress bar completes

# Wait for the thread to complete
thread.join()



# Calculate duration
end_time = time.time()
duration = end_time - start_time
minutes = duration / 60

# # Output the response from the API
# if response_json:
#     failed_items = [item for item in response_json.get('output', []) if "[Failed]" in item]
#     print(Fore.RED + "Failed items:")
#     for item in failed_items:
#         print(Fore.RED + item)

# Handling output based on the API response
if 'error' in response_json:
    print(Fore.RED + "Error during resource loading: " + response_json['error'])
else:
    # Output the response from the API
    failed_specs = 0
    failed_images = 0
    successful_specs = 0
    successful_images = 0
    if response_json:
        for item in response_json.get('output', []):
            if "spec:" in item:
                if "[Failed]" in item:
                    failed_specs += 1
                else :
                    successful_specs += 1
            elif "image:" in item:
                if "[Failed]" in item:
                    failed_images += 1
                else :
                    successful_images += 1

    print(Fore.CYAN + f"\nLoading completed ({minutes:.2f} minutes)")
    print(Fore.RESET + f"- Common specs")
    print(Fore.GREEN + f"- Successful: {successful_specs}" + Fore.RESET + f", Failed: {failed_specs}")
    print(Fore.RESET + f"- Common images")
    print(Fore.GREEN + f"- Successful: {successful_images}" + Fore.RESET + f", Failed: {failed_images}")

