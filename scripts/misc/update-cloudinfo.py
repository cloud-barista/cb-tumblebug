#!/usr/bin/env python3

# This script fetches csp regions and zones and updates the cloudinfo.yaml file with the latest data.
# It also compares the updated file with the original file and shows the diff.

# The script uses the following external APIs or CLIs:
# - Nominatim API: To fetch location details for each region. (https://nominatim.openstreetmap.org/search)
# - csp CLI: To fetch csp regions and zones and region description.

# The script can be run from the root of the repository using the following command:
# python3 update-cloudinfo.py

# The script outputs the following:
# - The updated cloudinfo-review.yaml file with the latest data.
# - The git diff output showing the changes in the updated file.

import subprocess
import yaml
import json
import requests

nominatim_base_url = 'https://nominatim.openstreetmap.org/search'

csp_commands = {
    'aws': {
        'regions': ['aws', 'ec2', 'describe-regions', '--all-regions', '--query', 'Regions[*].RegionName', '--output', 'json'],
        'zones': lambda region: ['aws', 'ec2', 'describe-availability-zones', '--region', region, '--query', 'AvailabilityZones[*].ZoneName', '--output', 'json']
    },
    'azure': {
        'regions': ['az', 'account', 'list-locations', '--query', '[].name', '--output', 'json'],
        'zones': lambda region: ['az', 'account', 'list-locations', '--query', f'[?name==`{region}`].{region}', '--output', 'json']
    },
    # Add more CSPs here
}

def run_command(command):
    try:
        print(f"Running command: {' '.join(command)}")
        result = subprocess.run(command, capture_output=True, text=True, check=True)
        return json.loads(result.stdout), None
    except subprocess.CalledProcessError as e:
        print(f"Error running command: {e}")
        return None, e

# Fetch regions and zones using each CSP CLI
def fetch_regions_and_zones(csp):
    regions_command = csp_commands[csp]['regions']
    print("Fetching regions and zones...")   
    print("(AuthFailure is because of permission issue (Opt-In). You need to opt-in regions)\n")

    regions, error = run_command(regions_command)
    if error:
        return {}

    region_zones = {}
    for region in regions:
        print(f"\n- Fetching availability zones for {region}...")
        zones_command = csp_commands[csp]['zones'](region)
        zones, error = run_command(zones_command)
        if not error:
            print(f"- {zones}")
        else:
            print(f"Failed to fetch zones for region {region}: {error}")
            zones = []
        region_zones[region] = zones

    return region_zones

# Fetch location details using Nominatim API
def fetch_location_details(display_name):
    try:
        params = {'q': display_name, 'format': 'json'}
        response = requests.get(nominatim_base_url, params=params)
        data = response.json()
    except requests.RequestException as e:
        print(f"Error fetching location details for {display_name}: {e}")
        return {'display': display_name, 'latitude': None, 'longitude': None}

    if data:
        return {
            'display': display_name,
            'latitude': float(data[0]['lat']),
            'longitude': float(data[0]['lon'])
        }
    else:
        return {'display': display_name, 'latitude': None, 'longitude': None}

# Fetch region description using csp CLI
def fetch_region_description(region_name):
    try:
        result = subprocess.run(
            ['aws', 'ssm', 'get-parameters-by-path', '--path', f'/aws/service/global-infrastructure/regions/{region_name}', '--query', "Parameters[?Name.contains(@,`longName`)].Value", '--output', 'text'],
            stdout=subprocess.PIPE, check=True)
        description = result.stdout.decode().strip()
    except subprocess.CalledProcessError as e:
        print(f"Error fetching region description for {region_name}: {e}\n\n")
        description = ""

    return description

# Compare and update the cloudinfo.yaml file with the latest data
def compare_and_update_yaml(csp, cloud_info, output_file_path, current_regions_and_zones):
    file_csp_regions = set(cloud_info[csp]['region'].keys())
    current_csp_regions = set(current_regions_and_zones.keys())

    missing_in_file = current_csp_regions - file_csp_regions
    extra_in_file = file_csp_regions - current_csp_regions

    missing_regions_msg = ', '.join(missing_in_file) if missing_in_file else "none"
    extra_regions_msg = ', '.join(extra_in_file) if extra_in_file else "none" 

    print()
    print(f"- Missing regions: {missing_regions_msg}")
    print(f"- Obsoleted regions: {extra_regions_msg}\n\n") 

    for region in current_csp_regions:
        if region in missing_in_file:
            desc = fetch_region_description(region)
            display = desc.split('(')[-1].rstrip(')') if '(' in desc else desc  # Improved parsing
            location_details = fetch_location_details(display)
            cloud_info[csp]['region'][region] = {
                'desc': desc,
                'location': location_details,
                'zone': current_regions_and_zones[region]
            }
            print(f"Added new region: {region}")
            print(f"Description: {desc}")
            print(f"Location: {location_details['display']} ({location_details['latitude']}, {location_details['longitude']})")
            print(f"Zones: {', '.join(current_regions_and_zones[region])}\n")
        else:
            cloud_info[csp]['region'][region]['zone'] = current_regions_and_zones[region]
            print(f"Updated zones for region: {region}")
            print(f"Zones: {', '.join(current_regions_and_zones[region])}\n")

    try:
        with open(output_file_path, 'w') as file:
            yaml.dump(cloud_info, file, default_flow_style=False, sort_keys=False)
    except IOError as e:
        print(f"Error writing to file {output_file_path}: {e}")

# Run git diff to show the changes in the updated file
def run_git_diff(original_file, updated_file):
    try:
        result = subprocess.run(['git', 'diff', '--no-index', '--color=always', original_file, updated_file], stdout=subprocess.PIPE, check=True, text=True)
        diff_output = result.stdout
        if diff_output:
            print(diff_output)
        else:
            print("No changes detected.")
    except subprocess.CalledProcessError as e:
        print(e.stdout)

def main():
    yaml_file_path = '../../assets/cloudinfo.yaml'
    output_file_path = './cloudinfo-review.yaml'

    try:
        with open(yaml_file_path, 'r') as file:
            cloud_info = yaml.safe_load(file)
    except FileNotFoundError as e:
        print(f"Error reading file {yaml_file_path}: {e}")
        return

    current_regions_and_zones = fetch_regions_and_zones('aws')
    
    compare_and_update_yaml('aws', cloud_info, output_file_path, current_regions_and_zones)

    run_git_diff(yaml_file_path, output_file_path)

if __name__ == "__main__":
    main()
