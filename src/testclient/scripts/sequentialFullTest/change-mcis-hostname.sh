#!/bin/bash

SECONDS=0

echo "[Check jq package (if not, install)]"
if ! dpkg-query -W -f='${Status}' jq | grep "ok installed"; then sudo apt install -y jq; fi

TestSetFile=${4:-../testSet.env}
if [ ! -f "$TestSetFile" ]; then
	echo "$TestSetFile does not exist."
	exit
fi
source $TestSetFile
source ../conf.env

echo "####################################################################"
echo "## Command (SSH) to MCIS to change-mcis-hostname"
echo "####################################################################"

CSP=${1}
REGION=${2:-1}
POSTFIX=${3:-developer}

source ../common-functions.sh
getCloudIndex $CSP

MCISID=${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX}

if [ "${INDEX}" == "0" ]; then
	# MCISPREFIX=avengers
	MCISID=${MCISPREFIX}-${POSTFIX}
fi

MCISINFO=$(curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NSID/mcis/${MCISID})
VMARRAY=$(jq -r '.vm' <<<"$MCISINFO")

for row in $(echo "${VMARRAY}" | jq -r '.[] | @base64'); do
	_jq() {
		echo ${row} | base64 --decode | jq -r ${1}
	}

    VMID=$(_jq '.id')
	connectionName=$(_jq '.connectionName')
    publicIP=$(_jq '.publicIP')
    cloudType=$(_jq '.location.cloudType')
    
    echo "VMID: $VMID"
    echo "connectionName: $connectionName"
    echo "publicIP: $publicIP"

    getCloudIndexGeneral $cloudType

    ChangeHostCMD="sudo hostnamectl set-hostname ${GeneralINDEX}-${connectionName}-${publicIP}; sudo hostname -f"
    ./command-mcis-vm-custom.sh "${1}" "${2}" "${3}" "${4}" "${VMID}" "${ChangeHostCMD}" &
done
wait

echo "Done!"
duration=$SECONDS

printElapsed $@
echo ""

./command-mcis.sh "$@"
