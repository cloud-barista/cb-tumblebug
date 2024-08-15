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
echo "## Command (SSH) to MCI "
echo "####################################################################"

CSP=${1}
REGION=${2:-1}
POSTFIX=${3:-developer}

source ../common-functions.sh
getCloudIndex $CSP

MCIID=${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX}

if [ "${INDEX}" == "0" ]; then
	# MCIPREFIX=avengers
	MCIID=${POSTFIX}
fi

MCIINFO=$(curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NSID/mci/${MCIID}?option=status)
VMARRAY=$(jq -r '.status.vm' <<<"$MCIINFO")
MASTERIP=$(jq -r '.status.masterIp' <<<"$MCIINFO")
MASTERVM=$(jq -r '.status.masterVmId' <<<"$MCIINFO")

echo "MASTERIP: $MASTERIP"
echo "MASTERVM: $MASTERVM"
echo "VMARRAY: $VMARRAY"

IPLIST=""

for row in $(echo "${VMARRAY}" | jq -r '.[] | @base64'); do
	_jq() {
		echo ${row} | base64 --decode | jq -r ${1}
	}

	IPLIST+=$(_jq '.publicIp')
	IPLIST+=" "
done

IPLIST=$(echo ${IPLIST})
echo "IPLIST: $IPLIST"

PRIVIPLIST=""

for row in $(echo "${VMARRAY}" | jq -r '.[] | @base64'); do
	_jq() {
		echo ${row} | base64 --decode | jq -r ${1}
	}

	PRIVIPLIST+=$(_jq '.privateIp')
	PRIVIPLIST+=" "
done

PRIVIPLIST=$(echo ${PRIVIPLIST})
echo "PRIVIPLIST: $PRIVIPLIST"



LAUNCHCMD="sudo scope stop"
echo "Stopping Weavescope for master node if exist..."
curl -H "${AUTH}" -sX POST http://$TumblebugServer/tumblebug/ns/$NSID/cmd/mci/$MCIID -H 'Content-Type: application/json' -d @- <<EOF
	{
	"command"        : "[${LAUNCHCMD}]"
	}
EOF

LAUNCHCMD="sudo scope launch $IPLIST $PRIVIPLIST"

echo "Launching Weavescope for master node..."
curl -H "${AUTH}" -sX POST http://$TumblebugServer/tumblebug/ns/$NSID/cmd/mci/$MCIID/vm/$MASTERVM -H 'Content-Type: application/json' -d @- <<EOF
	{
	"command"        : "[${LAUNCHCMD}]"
	}
EOF
#LAUNCHCMD="sudo scope launch $MASTERIP"

LAUNCHCMD="sudo scope launch $MASTERIP $PRIVIPLIST"

echo ""
echo "[MCI Weavescope: master node only] Access to"
echo " $MASTERIP:4040/#!/state/{\"contrastMode\":true,\"topologyId\":\"containers-by-hostname\"}"
echo ""
echo "Working on clustring..."

echo "Launching Weavescope for the other nodes..."
curl -H "${AUTH}" -sX POST http://$TumblebugServer/tumblebug/ns/$NSID/cmd/mci/$MCIID -H 'Content-Type: application/json' -d @- <<EOF
	{
	"command"        : "[${LAUNCHCMD}]"
	}
EOF

echo "Done!"
duration=$SECONDS

printElapsed $@
echo ""

echo "[MCI Weavescope: complete cluster] Access to"
echo " $MASTERIP:4040/#!/state/{\"contrastMode\":true,\"topologyId\":\"containers-by-hostname\"}"
echo ""
