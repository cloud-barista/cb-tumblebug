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
echo "## deploy-jitsi-to-mcis "
echo "####################################################################"

CSP=${1}
REGION=${2:-1}
POSTFIX=${3:-developer}

source ../common-functions.sh
getCloudIndex $CSP

MCISID=${CONN_CONFIG[$INDEX, $REGION]}-${POSTFIX}

PublicDNS=${5:-etri.cloud-barista.org}
EMAIL=${6:-shsonkorea@etri.re.kr}



if [ "${INDEX}" == "0" ]; then
	# MCISPREFIX=avengers
	MCISID=${MCISPREFIX}-${POSTFIX}
fi

MCISINFO=$(curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NS_ID/mcis/${MCISID}?action=status)
VMARRAY=$(jq -r '.status.vm' <<<"$MCISINFO")

echo "VMARRAY: $VMARRAY"

PublicIP=""
VMID=""

for row in $(echo "${VMARRAY}" | jq -r '.[] | @base64'); do
	_jq() {
		echo ${row} | base64 --decode | jq -r ${1}
	}

	VMID=$(_jq '.id')
	PublicIP=$(_jq '.public_ip')

	SetHostCMD="sudo -- sh -c \\\"echo $PublicIP $PublicDNS >> /etc/hosts\\\"; sudo hostnamectl set-hostname etri.cloud-barista.org; hostname -f"
	echo "SetHostCMD: $SetHostCMD"

	VAR1=$(curl -H "${AUTH}" -sX POST http://$TumblebugServer/tumblebug/ns/$NS_ID/cmd/mcis/$MCISID/vm/$VMID -H 'Content-Type: application/json' -d @- <<EOF
	{
	"command"        : "${SetHostCMD}"
	}
EOF
	)
	echo "${VAR1}" | jq ''
	echo ""

	InstallJitsiCMD="sudo sh -c \\\"echo 'deb https://download.jitsi.org stable/' > /etc/apt/sources.list.d/jitsi-stable.list\\\"; sudo wget -qO -  https://download.jitsi.org/jitsi-key.gpg.key | sudo apt-key add -; sudo echo \\\"jitsi-videobridge jitsi-videobridge/jvb-hostname string $PublicDNS \\\" | sudo debconf-set-selections; sudo echo \\\"jitsi-meet-web-config jitsi-meet/cert-choice select 'Generate a new self-signed certificate (You will later get a chance to obtain a Let's encrypt certificate)'\\\" | sudo debconf-set-selections; export DEBIAN_FRONTEND=noninteractive; sudo apt update > /dev/null; sudo apt-get --option=Dpkg::Options::=--force-confold --option=Dpkg::options::=--force-unsafe-io --assume-yes --quiet install jitsi-meet > /dev/null"
	echo "InstallJitsiCMD: $InstallJitsiCMD"

	VAR1=$(curl -H "${AUTH}" -sX POST http://$TumblebugServer/tumblebug/ns/$NS_ID/cmd/mcis/$MCISID/vm/$VMID -H 'Content-Type: application/json' -d @- <<EOF
	{
	"command"        : "${InstallJitsiCMD}"
	}
EOF
	)
	echo "${VAR1}" | jq ''
	echo ""

	# ConfigJitsiCMD="sudo echo $EMAIL | sudo /usr/share/jitsi-meet/scripts/install-letsencrypt-cert.sh; sudo -- sh -c \\\"echo DefaultLimitNOFILE=65000 >> /etc/systemd/system.conf\\\"; sudo -- sh -c \\\"echo DefaultLimitNPROC=65000 >> /etc/systemd/system.conf\\\"; sudo -- sh -c \\\"echo DefaultTasksMax=65000 >> /etc/systemd/system.conf\\\"; sudo cat /etc/systemd/system.conf; sudo systemctl daemon-reload; sudo systemctl restart prosody; sudo systemctl restart jicofo; sudo systemctl restart jitsi-videobridge2; sudo cat /proc/`sudo cat /var/run/jitsi-videobridge/jitsi-videobridge.pid`/limits"
	CertJitsiCMD="sudo echo $EMAIL | sudo /usr/share/jitsi-meet/scripts/install-letsencrypt-cert.sh"
	echo "CertJitsiCMD: $CertJitsiCMD"

	VAR1=$(curl -H "${AUTH}" -sX POST http://$TumblebugServer/tumblebug/ns/$NS_ID/cmd/mcis/$MCISID/vm/$VMID -H 'Content-Type: application/json' -d @- <<EOF
	{
	"command"        : "${CertJitsiCMD}"
	}
EOF
	)
	echo "${VAR1}" | jq ''
	echo ""

	ConfigJitsiCMD="sudo -- sh -c \\\"echo DefaultLimitNOFILE=65000 >> /etc/systemd/system.conf\\\"; sudo -- sh -c \\\"echo DefaultLimitNPROC=65000 >> /etc/systemd/system.conf\\\"; sudo -- sh -c \\\"echo DefaultTasksMax=65000 >> /etc/systemd/system.conf\\\"; sudo cat /etc/systemd/system.conf; sudo systemctl daemon-reload; sudo systemctl restart prosody; sudo systemctl restart jicofo; sudo systemctl restart jitsi-videobridge2; sudo cat /proc/\`sudo cat /var/run/jitsi-videobridge/jitsi-videobridge.pid\`/limits"
	echo "ConfigJitsiCMD: $ConfigJitsiCMD"

	VAR1=$(curl -H "${AUTH}" -sX POST http://$TumblebugServer/tumblebug/ns/$NS_ID/cmd/mcis/$MCISID/vm/$VMID -H 'Content-Type: application/json' -d @- <<EOF
	{
	"command"        : "${ConfigJitsiCMD}"
	}
EOF
	)
	echo "${VAR1}" | jq ''
	echo ""

done


echo "Done!"
duration=$SECONDS
echo "[CMD] $0"
echo "$(($duration / 60)) minutes and $(($duration % 60)) seconds elapsed."
echo ""

echo "[MCIS Jitsi: complete] Access to"
echo " https://${PublicDNS}"
echo ""
