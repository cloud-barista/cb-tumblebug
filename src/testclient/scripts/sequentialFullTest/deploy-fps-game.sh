#!/bin/bash


echo "####################################################################"
echo "## Deploy-Xonotic-FPS-Game-to-infra "
echo "####################################################################"

SECONDS=0
source ../init.sh


InfraINFO=$(curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NSID/infra/${InfraID}?option=status)
VMARRAY=$(jq -r '.status.vm' <<<"$InfraINFO")
MASTERIP=$(jq -r '.status.masterIp' <<<"$InfraINFO")
MASTERVM=$(jq -r '.status.masterVmId' <<<"$InfraINFO")

echo "MASTERIP: $MASTERIP"

LAUNCHCMD="sudo scope launch $IPLIST $PRIVIPLIST"
#echo $LAUNCHCMD

echo ""
echo "Installing Xonotic to Infra..."
#InstallFilePath="https://.../xonotic-0.8.2.zip"
InstallFilePath="https://z.xnz.me/xonotic/builds/xonotic-0.8.2.zip"
INSTALLCMD="sudo apt-get update > /dev/null; wget $InstallFilePath ; sudo apt install unzip -y; unzip xonotic-0.8.2.zip"
echo ""

VAR1=$(curl -H "${AUTH}" -sX POST http://$TumblebugServer/tumblebug/ns/$NSID/cmd/infra/$InfraID -H 'Content-Type: application/json' -d @- <<EOF
	{
	"command"        : "[${INSTALLCMD}]"
	}
EOF
)
echo "${VAR1}" | jq '.'
echo ""

LAUNCHCMD="cd Xonotic/; nohup ./xonotic-linux64-dedicated 1>server.log 2>&1 &"

echo "Launching Xonotic for master node..."
curl -H "${AUTH}" -sX POST http://$TumblebugServer/tumblebug/ns/$NSID/cmd/infra/$InfraID/vm/$MASTERVM -H 'Content-Type: application/json' -d @- <<EOF
	{
	"command"        : "[${LAUNCHCMD}]"
	}
EOF


echo "Done!"
duration=$SECONDS

printElapsed $@
echo ""

echo "[Infra Xonotic: complete] Access to"
echo " $MASTERIP:26000"
echo ""
