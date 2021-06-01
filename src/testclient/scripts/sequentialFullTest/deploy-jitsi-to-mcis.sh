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
echo "## deploy-jitsi-to-mcis "
echo "####################################################################"

CSP=${1}
REGION=${2:-1}
POSTFIX=${3:-developer}

source ../common-functions.sh
getCloudIndex $CSP

MCISID=${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX}

EMAIL=${5}
PublicDNS=${6}




if [ "${INDEX}" == "0" ]; then
	# MCISPREFIX=avengers
	MCISID=${MCISPREFIX}-${POSTFIX}
fi

if [ -z "$EMAIL" ]; then
	echo "[Warning] Provide your E-MAIL (ex: xxx@cloudbarista.org) to 5th parameter"
	echo "E-MAIL address will be used to issue a Certificate (https) for JITSI"
	exit
fi

if [ -z "$PublicDNS" ]; then
	echo "[Warning] Provide your PublicDNS-RecordName (ex: xxx.cloud-barista.org) to 6th parameter"
	echo "PublicDNS-RecordName will be access point for JITSI Server"
	exit
fi



MCISINFO=$(curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NSID/mcis/${MCISID}?action=status)
VMARRAY=$(jq -r '.status.vm' <<<"$MCISINFO")

echo "VMARRAY: $VMARRAY"

PublicIP=""
VMID=""

for row in $(echo "${VMARRAY}" | jq -r '.[] | @base64'); do
	_jq() {
		echo ${row} | base64 --decode | jq -r ${1}
	}

	VMID=$(_jq '.id')
	PublicIP=$(_jq '.publicIp')

	SetHostCMD="sudo -- sh -c \\\"echo $PublicIP $PublicDNS >> /etc/hosts\\\"; sudo hostnamectl set-hostname $PublicDNS; hostname -f"
	echo "SetHostCMD: $SetHostCMD"

	VAR1=$(curl -H "${AUTH}" -sX POST http://$TumblebugServer/tumblebug/ns/$NSID/cmd/mcis/$MCISID/vm/$VMID -H 'Content-Type: application/json' -d @- <<EOF
	{
	"command"        : "${SetHostCMD}"
	}
EOF
	)
	echo "${VAR1}" | jq ''
	echo ""


	InstallJitsiDependencyCMD="sudo sh -c \\\"echo 'deb https://download.jitsi.org stable/' > /etc/apt/sources.list.d/jitsi-stable.list\\\"; sudo wget -qO -  https://download.jitsi.org/jitsi-key.gpg.key | sudo apt-key add -; sudo apt-get update > /dev/null; sudo apt-get -y install apt-utils; sudo DEBIAN_FRONTEND=noninteractive apt-get -y install openjdk-8-jre-headless libssl1.1"
	echo "InstallJitsiDependencyCMD: $InstallJitsiDependencyCMD"

	VAR1=$(curl -H "${AUTH}" -sX POST http://$TumblebugServer/tumblebug/ns/$NSID/cmd/mcis/$MCISID/vm/$VMID -H 'Content-Type: application/json' -d @- <<EOF
	{
	"command"        : "${InstallJitsiDependencyCMD}"
	}
EOF
	)
	echo "${VAR1}" | jq ''
	echo ""

	InstallJitsiCMD="sudo echo \\\"jitsi-videobridge jitsi-videobridge/jvb-hostname string $PublicDNS\\\" | sudo debconf-set-selections; sudo echo \\\"jitsi-meet-web-config jitsi-meet/cert-choice select 'Generate a new self-signed certificate (You will later get a chance to obtain a Let's encrypt certificate)'\\\" | sudo debconf-set-selections; sudo DEBIAN_FRONTEND=noninteractive apt-get --option=Dpkg::Options::=--force-confold --option=Dpkg::options::=--force-unsafe-io --assume-yes --quiet install jitsi-meet"
	echo "InstallJitsiCMD: $InstallJitsiCMD"

	VAR1=$(curl -H "${AUTH}" -sX POST http://$TumblebugServer/tumblebug/ns/$NSID/cmd/mcis/$MCISID/vm/$VMID -H 'Content-Type: application/json' -d @- <<EOF
	{
	"command"        : "${InstallJitsiCMD}"
	}
EOF
	)
	echo "${VAR1}" | jq ''
	echo ""

	# ConfigJitsiCMD="sudo echo $EMAIL | sudo /usr/share/jitsi-meet/scripts/install-letsencrypt-cert.sh; sudo -- sh -c \\\"echo DefaultLimitNOFILE=65000 >> /etc/systemd/system.conf\\\"; sudo -- sh -c \\\"echo DefaultLimitNPROC=65000 >> /etc/systemd/system.conf\\\"; sudo -- sh -c \\\"echo DefaultTasksMax=65000 >> /etc/systemd/system.conf\\\"; sudo cat /etc/systemd/system.conf; sudo systemctl daemon-reload; sudo systemctl restart prosody; sudo systemctl restart jicofo; sudo systemctl restart jitsi-videobridge2; sudo cat /proc/`sudo cat /var/run/jitsi-videobridge/jitsi-videobridge.pid`/limits"
	CertJitsiCMD="sudo echo $EMAIL | sudo DEBIAN_FRONTEND=noninteractive /usr/share/jitsi-meet/scripts/install-letsencrypt-cert.sh"
	echo "CertJitsiCMD: $CertJitsiCMD"

	VAR1=$(curl -H "${AUTH}" -sX POST http://$TumblebugServer/tumblebug/ns/$NSID/cmd/mcis/$MCISID/vm/$VMID -H 'Content-Type: application/json' -d @- <<EOF
	{
	"command"        : "${CertJitsiCMD}"
	}
EOF
	)
	echo "${VAR1}" | jq ''
	echo ""

	ConfigJitsiCMD="sudo -- sh -c \\\"echo DefaultLimitNOFILE=65000 >> /etc/systemd/system.conf\\\"; sudo -- sh -c \\\"echo DefaultLimitNPROC=65000 >> /etc/systemd/system.conf\\\"; sudo -- sh -c \\\"echo DefaultTasksMax=65000 >> /etc/systemd/system.conf\\\"; sudo cat /etc/systemd/system.conf; sudo systemctl daemon-reload; sudo systemctl restart prosody; sudo systemctl restart jicofo; sudo systemctl restart jitsi-videobridge2; sudo cat /proc/\`sudo cat /var/run/jitsi-videobridge/jitsi-videobridge.pid\`/limits"
	echo "ConfigJitsiCMD: $ConfigJitsiCMD"

	VAR1=$(curl -H "${AUTH}" -sX POST http://$TumblebugServer/tumblebug/ns/$NSID/cmd/mcis/$MCISID/vm/$VMID -H 'Content-Type: application/json' -d @- <<EOF
	{
	"command"        : "${ConfigJitsiCMD}"
	}
EOF
	)
	echo "${VAR1}" | jq ''
	echo ""

done

# Ref: to add passwording
# https://www.digitalocean.com/community/tutorials/how-to-install-jitsi-meet-on-ubuntu-20-04
# https://sakwon.tistory.com/56

# sudo vim /etc/prosody/conf.avail/etri.cloud-barista.org.cfg.lua

# [chage authentication "anonymous" to "internal_plain"]
# VirtualHost "etri.cloud-barista.org"
#     -- enabled = false -- Remove this line to enable this host
#     authentication = "internal_plain"

# [last add]
# VirtualHost "guest.etri.cloud-barista.org"
#     authentication = "anonymous"
#     c2s_require_encryption = false


# sudo vim /etc/jitsi/meet/etri.cloud-barista.org-config.js

# [chage anonymousdomain]
# // When using authentication, domain for guest users.
# anonymousdomain: 'guest.etri.cloud-barista.org',


# sudo vim /etc/jitsi/jicofo/sip-communicator.properties

# [last add]
# org.jitsi.jicofo.auth.URL=XMPP:etri.cloud-barista.org

# sudo prosodyctl register shson etri.cloud-barista.org shsonpw

# sudo systemctl restart prosody.service
# sudo systemctl restart jicofo.service
# sudo systemctl restart jitsi-videobridge2.service

echo "Done!"
duration=$SECONDS

printElapsed $@
echo ""

echo "[MCIS Jitsi: complete] Access to"
echo " https://${PublicDNS}"
echo ""
