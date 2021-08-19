#!/bin/bash

SECONDS=0

echo "####################################################################"
echo "## Command (SSH) to MCIS to change-mcis-hostname"
echo "####################################################################"

source ../init.sh

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

    # ChangeHostCMD="sudo hostnamectl set-hostname ${GeneralINDEX}-${connectionName}-${publicIP}; sudo hostname -f"
    USERCMD="sudo hostnamectl set-hostname ${GeneralINDEX}-${VMID}; echo -n [Public IP: ; curl https://api.ipify.org ; echo -n ], [Hostname: ; hostname -f; echo -n ]"
	VAR1=$(
		curl -H "${AUTH}" -sX POST http://$TumblebugServer/tumblebug/ns/$NSID/cmd/mcis/$MCISID/vm/$VMID -H 'Content-Type: application/json' -d @- <<EOF
	{
	"command"        : "${USERCMD}"
	} 
EOF
	)
    echo "${VAR1}" | jq ''

done
wait

echo "Done!"
duration=$SECONDS

printElapsed $@
echo ""

