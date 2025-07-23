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
import shutil
import re


nominatim_base_url = 'https://nominatim.openstreetmap.org/search'

# Azure regions that DO NOT support VM creation (mostly disaster recovery regions)
AZURE_NON_VM_REGIONS = {
    'australiacentral2',
    'brazilsoutheast', 
    'francesouth',
    'germanynorth',
    'norwaywest',
    'southafricawest',
    'switzerlandwest',
    'uaecentral',
    'jioindiacentral',
    'brazilus',
    'eastus2euap',
    'eastusstg',
    'centraluseuap',
}

# Azure regions that DO support VM creation (primary regions)
AZURE_VM_REGIONS = {
    'australiacentral', 'australiaeast', 'australiasoutheast', 'brazilsouth', 'canadacentral', 
    'canadaeast', 'centralindia', 'centralus', 'eastasia', 'eastus2', 'eastus', 'francecentral', 
    'germanywestcentral', 'japaneast', 'japanwest', 'jioindiawest', 'koreacentral', 'koreasouth', 
    'northcentralus', 'northeurope', 'norwayeast', 'southafricanorth', 'southcentralus', 
    'southindia', 'southeastasia', 'swedencentral', 'switzerlandnorth', 'uaenorth', 'uksouth', 
    'ukwest', 'westcentralus', 'westeurope', 'westindia', 'westus2', 'westus3', 'westus', 
    'qatarcentral', 'israelcentral', 'polandcentral', 'italynorth', 'spaincentral', 
    'mexicocentral', 'austriaeast', 'chilecentral', 'malaysiawest', 'newzealandnorth', 
    'indonesiacentral'
}

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

csp_connection_names = {
    'gcp': 'gcp-asia-east1',
    'azure': 'azure-eastus',
    'aws': 'aws-us-east-1',
    'ibm': 'ibm-us-east',
    'alibaba': 'alibaba-us-east-1',
    'tencent': 'tencent-ap-singapore',
    'ncpvpc': 'ncpvpc-kr',
    'ktcloudvpc': 'ktcloudvpc-kr1',
    'nhncloud': 'nhncloud-kr1'
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

# Fetch regions and zones using Spider
def fetch_regions_and_zones_from_spider(csp, region_zones):
    connection_name = csp_connection_names[csp]

    print(f"\n[CSP: {csp}]")
    url = 'http://localhost:1024/spider/regionzone'
    headers = {'Content-Type': 'application/json'}
    data = {'ConnectionName': connection_name}
    response = requests.get(url, headers=headers, json=data)

    if response.status_code == 200:
        response = response.json()
        regions_info = response['regionzone']
        regions_info = sorted(regions_info, key=lambda x: x['Name'])

        if csp not in region_zones:
            region_zones[csp] = {}
        for region_info in regions_info:
            region_name = region_info['Name']
            print(f"{region_name}:")
            zone_list = region_info.get('ZoneList', None)
            if zone_list is not None: 
                zones = [zone['Name'] for zone in zone_list]
                zones.sort()
            else:
                zones = [] 
            region_zones[csp][region_name] = zones
            print(f"{zones}")
    else:
        print(f"Failed to fetch {csp} regions and zones: {response.text}")
    return region_zones


# Fetch regions and zones using each CSP CLI
def fetch_regions_and_zones(csp, region_zones):
    regions_command = csp_commands[csp]['regions']
    # print("(AuthFailure is because of permission issue (Opt-In). You need to opt-in regions)\n")
    print(f"\n[CSP: {csp}]")

    regions, error = run_command(regions_command)
    if error:
        return {}
    
    if csp not in region_zones:
        region_zones[csp] = {}

    regions.sort()
    for region in regions:
        print(f"{region}:")
        zones_command = csp_commands[csp]['zones'](region)
        zones, error = run_command(zones_command)
        if not error:
            print(f"{zones}")
        else:
            print(f"Failed to fetch zones for region {region}: {error}")
            zones = []
        zones.sort()
        region_zones[csp][region] = zones

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

    # Check if AWS CLI is installed
    if shutil.which('aws') is None:
        print(f"AWS CLI is not installed. Cannot fetch region description for {region_name}.")
        return f"No description available for {region_name}"

    try:
        result = subprocess.run(
            ['aws', 'ssm', 'get-parameters-by-path', '--path', f'/aws/service/global-infrastructure/regions/{region_name}', '--query', "Parameters[?Name.contains(@,`longName`)].Value", '--output', 'text'],
            stdout=subprocess.PIPE, check=True)
        description = result.stdout.decode().strip()
    except subprocess.CalledProcessError as e:
        print(f"Error fetching region description for {region_name}: {e}\n\n")
        description = ""

    return description

# Write YAML with Azure non-VM regions commented out
def write_yaml_with_azure_comments(cloud_info, output_file_path):
    """
    Write YAML file with Azure non-VM regions commented out.
    Modifies description field and maintains original indentation.
    """
    try:
        # First, modify the cloud_info to add disaster recovery info to descriptions
        if 'azure' in cloud_info.get('cloud', {}):
            azure_regions = cloud_info['cloud']['azure']['region']
            for region_name in azure_regions:
                if region_name in AZURE_NON_VM_REGIONS:
                    # Add disaster recovery info to description (keep it short to avoid line breaks)
                    current_desc = azure_regions[region_name].get('description', region_name)
                    azure_regions[region_name]['description'] = f"{current_desc} - DR region (No VM support)"
        
        # Write YAML
        with open(output_file_path, 'w') as file:
            yaml.dump(cloud_info, file, default_flow_style=False, sort_keys=False)
        
        # Read the file and comment out Azure non-VM regions
        with open(output_file_path, 'r') as file:
            content = file.read()
        
        modified_lines = content.split('\n')
        
        # Process Azure regions that don't support VM creation
        if 'azure' in cloud_info.get('cloud', {}):
            lines = content.split('\n')
            modified_lines = []
            in_azure_region = False
            in_non_vm_region = False
            current_region = None
            region_indent = 0
            
            for line in lines:
                # Check if we're in Azure section
                if re.match(r'^  azure:', line):
                    in_azure_region = True
                    modified_lines.append(line)
                    continue
                    
                # Check if we've left Azure section
                if in_azure_region and re.match(r'^  \w+:', line) and not re.match(r'^    ', line):
                    in_azure_region = False
                    in_non_vm_region = False
                    
                # If we're in Azure region section
                if in_azure_region and re.match(r'^      \w+:', line):
                    # Extract region name
                    region_match = re.match(r'^( +)(\w+):', line)
                    if region_match:
                        region_indent = len(region_match.group(1))
                        current_region = region_match.group(2)
                        
                        # Check if this region should be commented
                        if current_region in AZURE_NON_VM_REGIONS:
                            in_non_vm_region = True
                            # Comment out the region line maintaining original indentation
                            modified_lines.append(f"      # {current_region}:")
                        else:
                            in_non_vm_region = False
                            modified_lines.append(line)
                    else:
                        modified_lines.append(line)
                        
                # If we're in a non-VM region, comment out all lines but preserve exact indentation
                elif in_non_vm_region and line.startswith(' ' * region_indent):
                    if line.strip():  # Only process non-empty lines
                        # Add # and space at the very beginning, preserving all original spaces
                        modified_lines.append(f"      #{line[6:]}")
                    else:
                        modified_lines.append('')  # Keep empty lines as is
                    
                # Check if we've left the current region
                elif in_non_vm_region and line.strip() and not line.startswith(' ' * region_indent):
                    in_non_vm_region = False
                    modified_lines.append(line)
                    
                else:
                    modified_lines.append(line)
        
        # Write modified content back
        with open(output_file_path, 'w') as file:
            file.write('\n'.join(modified_lines))
        
        # Add header comment to the file
        add_header_comment(output_file_path)
                
    except IOError as e:
        print(f"Error writing to file {output_file_path}: {e}")

def add_header_comment(file_path):
    """
    Add header comment to the beginning of the YAML file
    """
    header_comment = """# Configuration for Cloud Service Providers (CSPs)
# This file is used to define the CSPs and their regions.

# The file is in YAML format and contains the following fields:
# cloud: Top level key
#   <csp>: Name of the CSP
#     description: Description of the CSP
#     driver: Name of the driver library file (a prepared CB-Spider Driver)
#     link: 
#     -URLs to the official documentation of the CSP
#     region: List of regions
#       <region>:
#         description: Description of the region
#         location: Location details of the region
#           display: Display name
#           latitude: Latitude
#           longitude: Longitude
#         zone: List of availability zones in the region
#           <zone>:
#           - <ID/Name of the availability zon>

# Note: Special regions not supporting VM provisioning are disabled. Enable them by removing the comment.

"""
    
    try:
        # Read existing content
        with open(file_path, 'r') as file:
            content = file.read()
        
        # Write header + content
        with open(file_path, 'w') as file:
            file.write(header_comment + content)
            
    except IOError as e:
        print(f"Error adding header comment to {file_path}: {e}")
        
# Compare and update the cloudinfo.yaml file with the latest data
def compare_and_update_yaml(cloud_info, output_file_path, region_zones):
    current_regions_and_zones = region_zones
    csps = set(current_regions_and_zones.keys())

    if "cloud" not in cloud_info:
        cloud_info["cloud"] = {}

    for csp in csps:
        file_csp_regions = set(cloud_info["cloud"][csp]['region'].keys())
        current_csp_regions = set(current_regions_and_zones[csp].keys())

        missing_in_file = current_csp_regions - file_csp_regions
        extra_in_file = file_csp_regions - current_csp_regions

        missing_regions_msg = ', '.join(missing_in_file) if missing_in_file else "none"
        extra_regions_msg = ', '.join(extra_in_file) if extra_in_file else "none" 

        print()
        print(f"- Missing regions: {missing_regions_msg}")
        print(f"- Obsoleted regions: {extra_regions_msg}")
        
        # Azure specific: show non-VM regions
        if csp == 'azure':
            non_vm_regions = current_csp_regions & AZURE_NON_VM_REGIONS
            if non_vm_regions:
                print(f"- Non-VM regions (will be commented): {', '.join(non_vm_regions)}")
        print("\n")

        for region in current_csp_regions:
            if region in missing_in_file:
                desc = fetch_region_description(region)
                display = desc.split('(')[-1].rstrip(')') if '(' in desc else desc  # Improved parsing
                location_details = fetch_location_details(display)
                cloud_info["cloud"][csp]['region'][region] = {
                    'desc': desc,
                    'location': location_details,
                    'zone': current_regions_and_zones[csp][region]
                }
                print(f"Added new region: {region}")
                print(f"Description: {desc}")
                print(f"Location: {location_details['display']} ({location_details['latitude']}, {location_details['longitude']})")
                print(f"Zones: {', '.join(current_regions_and_zones[csp][region])}\n")
            else:
                cloud_info["cloud"][csp]['region'][region]['zone'] = current_regions_and_zones[csp][region]
                print(f"Updated zones for region: {region}")
                print(f"Zones: {', '.join(current_regions_and_zones[csp][region])}\n")

    # Write YAML with Azure non-VM regions commented out
    write_yaml_with_azure_comments(cloud_info, output_file_path)

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
    
    # Global variable to store region and zones for all CSPs
    region_zones = {}

    # region_zones = fetch_regions_and_zones('aws', region_zones)
    # region_zones = fetch_regions_and_zones_from_spider('azure', region_zones)
    # region_zones = fetch_regions_and_zones_from_spider('tencent', region_zones)
    # region_zones = fetch_regions_and_zones_from_spider('ibm', region_zones)

    target_csp = set(csp_connection_names.keys())
    for csp in target_csp:
        region_zones = fetch_regions_and_zones_from_spider(csp, region_zones)
    
    # region_zones = fetch_regions_and_zones('aws', region_zones)

    compare_and_update_yaml(cloud_info, output_file_path, region_zones)

    run_git_diff(yaml_file_path, output_file_path)

if __name__ == "__main__":
    main()
