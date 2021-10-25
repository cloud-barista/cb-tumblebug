#!/bin/bash

# Assign default values for network parameters
DEFAULT_NETWORK_INTERFACE="cbnet0"
DEFAULT_POD_NETWORK_CIDR="10.77.0.0/16"

NETWORK_INTERFACE=$DEFAULT_NETWORK_INTERFACE
POD_NETWORK_CIDR=$DEFAULT_POD_NETWORK_CIDR

CSP=${1}
REGION=${2:-1}
POSTFIX=${3:-developer}
TestSetFile=${4:-../testSet.env}

echo "Original parameters: $@"

# Update values for network parameters by named input parameters (i, c)
while getopts ":f:n:h:" opt; do
  case $opt in
    f) CSP="all"
       REGION="1"  
       TestSetFile="$OPTARG"
    ;;
    n) POSTFIX="$OPTARG"
    ;;
    h) echo "How to use '-h' (ex: ./${0##*/} -c ../testSet.env -n myname)"
       exit 0
    ;;
    \?) echo "Invalid option -$OPTARG (Use: -i for NETWORK_INTERFACE, -c for POD_NETWORK_CIDR)" >&2
        exit 0
    ;;
  esac
done

set -- "$CSP $REGION $POSTFIX $TestSetFile"

CSP=${1}
REGION=${2:-1}
POSTFIX=${3:-developer}
TestSetFile=${4:-../testSet.env}

# MASTER_IP_ADDRESS=$(ifconfig ${NETWORK_INTERFACE} | grep "inet " | awk '{print $2}')

# printf "[Network env variables for this Kubernetes cluster installation]\nNETWORK_INTERFACE=%s\nMASTER_IP_ADDRESS=%s\nPOD_NETWORK_CIDR=%s\n\n" "$NETWORK_INTERFACE" "$MASTER_IP_ADDRESS" "$POD_NETWORK_CIDR"

# if [ -z "$MASTER_IP_ADDRESS" ]
# then
#       echo "Warning! can not find NETWORK_INTERFACE $NETWORK_INTERFACE from ifconfig."
#       echo ""
#       echo "You need to provide an appropriate network interface."
#       echo "Please check ifconfig and find an interface (Ex: ens3, ens4, eth0, ...)"
#       echo "Then, provide the interface to this script with parameter option '-i' (ex: ./${0##*/} -i ens3)"
#       echo ""
#       echo "See you again.. :)"
#       exit 0
# fi




echo "Modified parameters: $@"