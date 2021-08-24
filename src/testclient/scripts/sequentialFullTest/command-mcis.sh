#!/bin/bash

echo "####################################################################"
echo "## Command (SSH) to MCIS with a user command"
echo "####################################################################"

source ../init.sh

USERCMD=$OPTION01
VMID=$OPTION02

if [ -z "$USERCMD" ]; then
	USERCMD="echo -n [Public IP: ; curl https://api.ipify.org ; echo -n ], [Hostname: ; hostname ; echo -n ]"
fi

if [ -z "$VMID" ]; then

	VAR1=$(
		curl -H "${AUTH}" -sX POST http://$TumblebugServer/tumblebug/ns/$NSID/cmd/mcis/$MCISID -H 'Content-Type: application/json' -d @- <<EOF
	{
	"command"        : "${USERCMD}"
	} 
EOF
	)
else
	VAR1=$(
		curl -H "${AUTH}" -sX POST http://$TumblebugServer/tumblebug/ns/$NSID/cmd/mcis/$MCISID/vm/$VMID -H 'Content-Type: application/json' -d @- <<EOF
	{
	"command"        : "${USERCMD}"
	} 
EOF
	)

fi

echo "${VAR1}" | jq ''
