#!/bin/bash


echo "####################################################################"
echo "## deploy-jitsi-to-infra (parameters: -x EMAIL -y PublicDNS)"
echo "####################################################################"

SECONDS=0

source ../init.sh

EMAIL=${OPTION01}
PublicDNS=${OPTION02}

if [ "${INDEX}" == "0" ]; then
	# InfraPREFIX=avengers
	InfraID=${POSTFIX}
fi

if [ -z "$EMAIL" ]; then
	echo "[Warning] Provide your E-MAIL (ex: xxx@cloudbarista.org) to -x parameter"
	echo "E-MAIL address will be used to issue a Certificate (https) for JITSI"
	exit
fi

if [ -z "$PublicDNS" ]; then
	echo "[Warning] Provide your PublicDNS-RecordName (ex: xxx.cloud-barista.org) to -y parameter"
	echo "PublicDNS-RecordName will be access point for JITSI Server"
	exit
fi



InfraINFO=$(curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NSID/infra/${InfraID}?option=status)
VMARRAY=$(jq -r '.status.vm' <<<"$InfraINFO")

echo "VMARRAY: $VMARRAY"

PublicIP=""
VMID=""

for row in $(echo "${VMARRAY}" | jq -r '.[] | @base64'); do
	_jq() {
		echo ${row} | base64 --decode | jq -r ${1}
	}

	VMID=$(_jq '.id')
	PublicIP=$(_jq '.publicIp')

done

CMD="wget https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/scripts/usecases/jitsi/startServer.sh; chmod +x ~/startServer.sh; sudo ~/startServer.sh $PublicIP $PublicDNS $EMAIL"
echo "CMD: $CMD"

VAR1=$(curl -H "${AUTH}" -sX POST http://$TumblebugServer/tumblebug/ns/$NSID/cmd/infra/$InfraID -H 'Content-Type: application/json' -d @- <<EOF
	{
	"command"        : "[${CMD}]"
	}
EOF
)
echo "${VAR1}" | jq '.'

echo "Done!"
duration=$SECONDS

printElapsed $@
echo ""

echo "[Infra Jitsi: complete] Access to"
echo " ${PublicDNS}"
echo ""
