#!/usr/bin/env python3

import requests
import yaml
import os
import base64
import time
import sys
from tqdm import tqdm
import threading

# Configuration
TUMBLEBUG_SERVER = 'localhost:1323'
API_USERNAME = 'default'
API_PASSWORD = 'default'
AUTH = f"Basic {base64.b64encode(f'{API_USERNAME}:{API_PASSWORD}'.encode()).decode()}"

CBTUMBLEBUG_ROOT = os.getenv('CBTUMBLEBUG_ROOT', os.path.abspath(os.path.join(os.path.dirname(__file__), '..')))
CRED_FILE_NAME = "credentials.yaml"
CRED_PATH = os.path.join(os.path.expanduser('~'), '.cloud-barista')

# Check for credential path
if not os.path.exists(CRED_PATH):
    print("Error: CRED_PATH does not exist. Please run scripts/genCredential.sh first.")
    sys.exit(1)
elif not os.path.isfile(os.path.join(CRED_PATH, CRED_FILE_NAME)):
    print("Error: CRED_FILE_NAME does not exist. Please check if it has been generated.")
    sys.exit(1)


# Wait for user input to proceed
print("\nRegistering credentials and Loading common Specs and Images takes time")
if input('Do you want to proceed ? (y/n) : ').lower() not in ['y', 'yes']:
    print("Cancel [{}]".format(' '.join(sys.argv)))
    print("See you soon. :)")
    sys.exit(0)


# Load credentials from YAML file
with open(os.path.join(CRED_PATH, CRED_FILE_NAME), 'r') as file:
    cred_data = yaml.safe_load(file)['credentialholder']['admin']

print(f"\n[Registering all valid credentials for all cloud regions]")
# Register credentials to TumblebugServer
headers = {'Authorization': AUTH, 'Content-Type': 'application/json'}
for provider, credentials in cred_data.items():
    if all(credentials.values()):
        print(f"{provider}: registering credentials for all cloud regions")
        credential_payload = {
            "credentialHolder": "admin",
            "keyValueInfoList": [{"key": k, "value": v} for k, v in credentials.items()],
            "providerName": provider
        }
        response = requests.post(f"http://{TUMBLEBUG_SERVER}/tumblebug/credential", json=credential_payload, headers=headers)
        print(response.json())
    else:
        print(f"{provider}: Incomplete credential data, Skip")

# Function to perform the HTTP request
def load_resources():
    global response_json
    response = requests.get(f"http://{TUMBLEBUG_SERVER}/tumblebug/loadCommonResource", headers=headers)
    response_json = response.json()
    event.set()  # Signal that the request is complete

# Start time
start_time = time.time()

# Event object to signal the request completion
event = threading.Event()

# Start the network request in a separate thread
thread = threading.Thread(target=load_resources)
thread.start()

# Expected duration and progress bar
expected_duration = 240  # 240 seconds for the demo purpose
with tqdm(total=expected_duration, desc="Loading Resources", unit='s') as pbar:
    while not event.is_set():
        time.sleep(1)
        pbar.update(1)
    pbar.update(expected_duration - pbar.n)  # Complete the progress bar if not fully updated

# Wait for the thread to complete
thread.join()

# Output the response from the API
print(response_json)
print("Loading completed.")

# Calculate duration
end_time = time.time()
duration = end_time - start_time

# Display the response data and total duration
print(f"Total duration: {duration} seconds.")
