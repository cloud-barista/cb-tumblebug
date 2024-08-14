#!/bin/bash


echo "####################################################################"
echo "## Deploy-Xonotic-FPS-Game-to-mci "
echo "####################################################################"

SECONDS=0
source ../init.sh


MCIINFO=$(curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NSID/mci/${MCIID}?option=status)
VMARRAY=$(jq -r '.status.vm' <<<"$MCIINFO")
MASTERIP=$(jq -r '.status.masterIp' <<<"$MCIINFO")
MASTERVM=$(jq -r '.status.masterVmId' <<<"$MCIINFO")

echo "MASTERIP: $MASTERIP"

LAUNCHCMD="sudo scope launch $IPLIST $PRIVIPLIST"
#echo $LAUNCHCMD

echo ""
echo "Installing Xonotic to MCI..."
#InstallFilePath="https://.../xonotic-0.8.2.zip"
InstallFilePath="https://z.xnz.me/xonotic/builds/xonotic-0.8.2.zip"
INSTALLCMD="sudo apt-get update > /dev/null; wget $InstallFilePath ; sudo apt install unzip -y; unzip xonotic-0.8.2.zip"
echo ""

VAR1=$(curl -H "${AUTH}" -sX POST http://$TumblebugServer/tumblebug/ns/$NSID/cmd/mci/$MCIID -H 'Content-Type: application/json' -d @- <<EOF
	{
	"command"        : "[${INSTALLCMD}]"
	}
EOF
)
echo "${VAR1}" | jq ''
echo ""

LAUNCHCMD="cd Xonotic/; nohup ./xonotic-linux64-dedicated 1>server.log 2>&1 &"

echo "Launching Xonotic for master node..."
curl -H "${AUTH}" -sX POST http://$TumblebugServer/tumblebug/ns/$NSID/cmd/mci/$MCIID/vm/$MASTERVM -H 'Content-Type: application/json' -d @- <<EOF
	{
	"command"        : "[${LAUNCHCMD}]"
	}
EOF


echo "Done!"
duration=$SECONDS

printElapsed $@
echo ""

echo "[MCI Xonotic: complete] Access to"
echo " $MASTERIP:26000"
echo ""
