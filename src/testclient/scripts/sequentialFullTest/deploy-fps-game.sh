#!/bin/bash


echo "####################################################################"
echo "## Deploy-Xonotic-FPS-Game-to-mcis "
echo "####################################################################"

SECONDS=0
source ../init.sh


MCISINFO=$(curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NSID/mcis/${MCISID}?option=status)
VMARRAY=$(jq -r '.status.vm' <<<"$MCISINFO")
MASTERIP=$(jq -r '.status.masterIp' <<<"$MCISINFO")
MASTERVM=$(jq -r '.status.masterVmId' <<<"$MCISINFO")

echo "MASTERIP: $MASTERIP"

LAUNCHCMD="sudo scope launch $IPLIST $PRIVIPLIST"
#echo $LAUNCHCMD

echo ""
echo "Installing Xonotic to MCIS..."
#InstallFilePath="https://.../xonotic-0.8.2.zip"
InstallFilePath="https://z.xnz.me/xonotic/builds/xonotic-0.8.2.zip"
INSTALLCMD="sudo apt-get update > /dev/null; wget $InstallFilePath ; sudo apt install unzip -y; unzip xonotic-0.8.2.zip"
echo ""

VAR1=$(curl -H "${AUTH}" -sX POST http://$TumblebugServer/tumblebug/ns/$NSID/cmd/mcis/$MCISID -H 'Content-Type: application/json' -d @- <<EOF
	{
	"command"        : "${INSTALLCMD}"
	}
EOF
)
echo "${VAR1}" | jq ''
echo ""

LAUNCHCMD="cd Xonotic/; ./xonotic-linux64-dedicated &"

echo "Launching Xonotic for master node..."
curl -H "${AUTH}" -sX POST http://$TumblebugServer/tumblebug/ns/$NSID/cmd/mcis/$MCISID/vm/$MASTERVM -H 'Content-Type: application/json' -d @- <<EOF
	{
	"command"        : "${LAUNCHCMD}"
	}
EOF


echo "Done!"
duration=$SECONDS

printElapsed $@
echo ""

echo "[MCIS Xonotic: complete] Access to"
echo " $MASTERIP:26000"
echo ""
