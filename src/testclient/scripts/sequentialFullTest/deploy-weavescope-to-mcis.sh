#!/bin/bash

SECONDS=0

echo "####################################################################"
echo "## Command (SSH) to MCIS (deploy-weavescope-to-mcis)"
echo "####################################################################"

source ../init.sh

if [ "${INDEX}" == "0" ]; then
	# MCISPREFIX=avengers
	MCISID=${POSTFIX}
fi

MCISINFO=$(curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NSID/mcis/${MCISID}?option=status)
VMARRAY=$(jq -r '.status.vm' <<<"$MCISINFO")
MASTERIP=$(jq -r '.status.masterIp' <<<"$MCISINFO")
MASTERVM=$(jq -r '.status.masterVmId' <<<"$MCISINFO")

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

LAUNCHCMD="sudo scope launch $IPLIST $PRIVIPLIST"
#echo $LAUNCHCMD

echo ""
echo "Installing Weavescope to MCIS..."
ScopeInstallFile="git.io/scope"
ScopeInstallFile="https://gist.githubusercontent.com/seokho-son/bb2703ca49555f9afe0d0097894c74fa/raw/9eb65b296b85bc53f53af3e8733603d807fb9287/scope"
INSTALLCMD="sudo apt-get update > /dev/null;  sudo apt install docker.io -y; sudo curl -L $ScopeInstallFile -o /usr/local/bin/scope; sudo chmod a+x /usr/local/bin/scope"
echo ""

VAR1=$(curl -H "${AUTH}" -sX POST http://$TumblebugServer/tumblebug/ns/$NSID/cmd/mcis/$MCISID -H 'Content-Type: application/json' -d @- <<EOF
	{
	"command"        : "${INSTALLCMD}"
	}
EOF
)
echo "${VAR1}" | jq ''
echo ""

echo "Launching Weavescope for master node..."
curl -H "${AUTH}" -sX POST http://$TumblebugServer/tumblebug/ns/$NSID/cmd/mcis/$MCISID/vm/$MASTERVM -H 'Content-Type: application/json' -d @- <<EOF
	{
	"command"        : "${LAUNCHCMD}"
	}
EOF
#LAUNCHCMD="sudo scope launch $MASTERIP"

LAUNCHCMD="sudo scope launch $MASTERIP $PRIVIPLIST"

echo ""
echo "[MCIS Weavescope: master node only] Access to"
echo " $MASTERIP:4040/#!/state/{\"contrastMode\":true,\"topologyId\":\"containers-by-hostname\"}"
echo ""
echo "Working on clustring..."

echo "Launching Weavescope for the other nodes..."
curl -H "${AUTH}" -sX POST http://$TumblebugServer/tumblebug/ns/$NSID/cmd/mcis/$MCISID -H 'Content-Type: application/json' -d @- <<EOF
	{
	"command"        : "${LAUNCHCMD}"
	}
EOF

echo "Done!"
duration=$SECONDS

printElapsed $@
echo ""

echo "[MCIS Weavescope: complete cluster] Access to"
echo " $MASTERIP:4040/#!/state/{\"contrastMode\":true,\"topologyId\":\"containers-by-hostname\"}"
echo ""
