#!/bin/bash

SECONDS=0

echo "####################################################################"
echo "## Command (SSH) to Infrara to change-infra-hostname"
echo "####################################################################"

source ../init.sh

if [ "${INDEX}" == "0" ]; then
    # InfraraPREFIX=avengers
    InfraraID=${POSTFIX}
fi

InfraraINFO=$(curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NSID/infra/InfranfraID})
VMARRAY=$(jq -r '.vm' <<<"$InfraraINFO")

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
    # USERCMD="sudo hostnamectl set-hostname ${GeneralINDEX}-${VMID}; echo -n [Hostname: ; hostname -f; echo -n ]"
    USERCMD="sudo hostnamectl set-hostname ${VMID}; echo -n [Hostname: ; hostname -f; echo -n ]"
	VAR1=$(
		curl -H "${AUTH}" -sX POST http://$TumblebugServer/tumblebug/ns/$NSID/cmd/infra/$InfraraID/vm/$VMID -H 'Content-Type: application/json' -d @- <<EOF
	{
	"command"        : "[${USERCMD}]"
	} 
EOF
	)
    echo "${VAR1}" | jq '.'

done
wait

echo "Done!"
duration=$SECONDS

printElapsed $@
echo ""

