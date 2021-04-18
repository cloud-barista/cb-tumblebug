#!/bin/bash

SECONDS=0

echo "[Check jq package (if not, install)]"
if ! dpkg-query -W -f='${Status}' jq | grep "ok installed"; then sudo apt install -y jq; fi

TestSetFile=${4:-../testSet.env}

FILE=$TestSetFile
if [ ! -f "$FILE" ]; then
	echo "$FILE does not exist."
	exit
fi
source $TestSetFile
source ../conf.env
AUTH="Authorization: Basic $(echo -n $ApiUsername:$ApiPassword | base64)"

echo "####################################################################"
echo "## Command (SSH) to MCIS to change-mcis-hostname"
echo "####################################################################"

CSP=${1}
REGION=${2:-1}
POSTFIX=${3:-developer}

source ../common-functions.sh
getCloudIndex $CSP

MCISID=${CONN_CONFIG[$INDEX, $REGION]}-${POSTFIX}

if [ "${INDEX}" == "0" ]; then
	# MCISPREFIX=avengers
	MCISID=${MCISPREFIX}-${POSTFIX}
fi

MCISINFO=$(curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NS_ID/mcis/${MCISID})
VMARRAY=$(jq -r '.vm' <<<"$MCISINFO")

for row in $(echo "${VMARRAY}" | jq -r '.[] | @base64'); do
	_jq() {
		echo ${row} | base64 --decode | jq -r ${1}
	}

    VMID=$(_jq '.id')
	connectionName=$(_jq '.connectionName')
    publicIP=$(_jq '.publicIP')
    
    echo "connectionName: $connectionName"
    echo "publicIP: $publicIP"

    ChangeHostCMD="sudo hostnamectl set-hostname ${connectionName}-${publicIP}; sudo hostname -f"
    ./command-mcis-vm-custom.sh "$@" "${VMID}" "${ChangeHostCMD}"

done

echo "Done!"
duration=$SECONDS
echo "[CMD] ${_self}"
echo "$(($duration / 60)) minutes and $(($duration % 60)) seconds elapsed."
echo ""

./command-mcis.sh "$@"
