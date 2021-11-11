#!/bin/bash


echo "####################################################################"
echo "## deploy-jitsi-to-mcis (parameters: -x EMAIL -y PublicDNS)"
echo "####################################################################"

SECONDS=0

source ../init.sh

EMAIL=${OPTION01}
PublicDNS=${OPTION02}

if [ "${INDEX}" == "0" ]; then
	# MCISPREFIX=avengers
	MCISID=${POSTFIX}
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



MCISINFO=$(curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NSID/mcis/${MCISID}?option=status)
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

	# Set Hostname
	SetHostCMD="sudo -- sh -c \\\"echo $PublicIP $PublicDNS >> /etc/hosts\\\"; sudo hostnamectl set-hostname $PublicDNS; hostname -f"
	echo "SetHostCMD: $SetHostCMD"

	VAR1=$(curl -H "${AUTH}" -sX POST http://$TumblebugServer/tumblebug/ns/$NSID/cmd/mcis/$MCISID/vm/$VMID -H 'Content-Type: application/json' -d @- <<EOF
	{
	"command"        : "${SetHostCMD}"
	}
EOF
	)
	echo "${VAR1}" | jq . | sed 's/\\n/\n\t - /g'
	echo ""

	# Remove (if) existing packages to initialize jitsi
	CMD="sudo sudo apt autoremove -y jigasi jitsi-meet jitsi-meet-web-config jitsi-meet-prosody jitsi-meet-turnserver jitsi-meet-web jicofo jitsi-videobridge2"
	echo "CMD: $CMD"

	VAR1=$(curl -H "${AUTH}" -sX POST http://$TumblebugServer/tumblebug/ns/$NSID/cmd/mcis/$MCISID/vm/$VMID -H 'Content-Type: application/json' -d @- <<EOF
	{
	"command"        : "${CMD}"
	}
EOF
	)
	echo "${VAR1}" | jq . | sed 's/\\n/\n\t - /g'
	echo ""

	# Set prosody
	InstallJitsiCMD="echo deb http://packages.prosody.im/debian $(lsb_release -sc) main | sudo tee -a /etc/apt/sources.list; wget https://prosody.im/files/prosody-debian-packages.key -O- | sudo apt-key add -"
	echo "InstallJitsiCMD: $InstallJitsiCMD"

	VAR1=$(curl -H "${AUTH}" -sX POST http://$TumblebugServer/tumblebug/ns/$NSID/cmd/mcis/$MCISID/vm/$VMID -H 'Content-Type: application/json' -d @- <<EOF
	{
	"command"        : "${InstallJitsiCMD}"
	}
EOF
	)
	echo "${VAR1}" | jq . | sed 's/\\n/\n\t - /g'
	echo ""

	# InstallJitsiDependencyCMD
	InstallJitsiDependencyCMD="sudo sh -c \\\"echo 'deb https://download.jitsi.org stable/' > /etc/apt/sources.list.d/jitsi-stable.list\\\"; sudo wget -qO -  https://download.jitsi.org/jitsi-key.gpg.key | sudo apt-key add -; sudo apt-add-repository universe; sudo apt-get update > /dev/null; sudo apt-get -y install apt-utils; sudo apt-get -y install apt-transport-https; sudo DEBIAN_FRONTEND=noninteractive apt-get -y install openjdk-8-jre-headless libssl1.1"
	echo "InstallJitsiDependencyCMD: $InstallJitsiDependencyCMD"

	VAR1=$(curl -H "${AUTH}" -sX POST http://$TumblebugServer/tumblebug/ns/$NSID/cmd/mcis/$MCISID/vm/$VMID -H 'Content-Type: application/json' -d @- <<EOF
	{
	"command"        : "${InstallJitsiDependencyCMD}"
	}
EOF
	)
	echo "${VAR1}" | jq . | sed 's/\\n/\n\t - /g'
	echo ""

	# InstallJitsiCMD
	InstallJitsiCMD="sudo echo \\\"jitsi-videobridge jitsi-videobridge/jvb-hostname string $PublicDNS\\\" | sudo debconf-set-selections; sudo echo \\\"jitsi-meet-web-config jitsi-meet/cert-choice select 'Generate a new self-signed certificate (You will later get a chance to obtain a Let's encrypt certificate)'\\\" | sudo debconf-set-selections; sudo DEBIAN_FRONTEND=noninteractive apt-get --option=Dpkg::Options::=--force-confold --option=Dpkg::options::=--force-unsafe-io --assume-yes --quiet install jitsi-meet"
	echo "InstallJitsiCMD: $InstallJitsiCMD"

	VAR1=$(curl -H "${AUTH}" -sX POST http://$TumblebugServer/tumblebug/ns/$NSID/cmd/mcis/$MCISID/vm/$VMID -H 'Content-Type: application/json' -d @- <<EOF
	{
	"command"        : "${InstallJitsiCMD}"
	}
EOF
	)
	echo "${VAR1}" | jq . | sed 's/\\n/\n\t - /g'
	echo ""

	# CertJitsiCMD for letsencrypt-certificate (will need actual DNS record)
	CertJitsiCMD="sudo echo $EMAIL | sudo DEBIAN_FRONTEND=noninteractive /usr/share/jitsi-meet/scripts/install-letsencrypt-cert.sh"
	echo "CertJitsiCMD: $CertJitsiCMD"

	VAR1=$(curl -H "${AUTH}" -sX POST http://$TumblebugServer/tumblebug/ns/$NSID/cmd/mcis/$MCISID/vm/$VMID -H 'Content-Type: application/json' -d @- <<EOF
	{
	"command"        : "${CertJitsiCMD}"
	}
EOF
	)
	echo "${VAR1}" | jq . | sed 's/\\n/\n\t - /g'
	echo ""

	# ConfigJitsiCMD 
	ConfigJitsiCMD="sudo -- sh -c \\\"echo DefaultLimitNOFILE=65000 >> /etc/systemd/system.conf\\\"; sudo -- sh -c \\\"echo DefaultLimitNPROC=65000 >> /etc/systemd/system.conf\\\"; sudo -- sh -c \\\"echo DefaultTasksMax=65000 >> /etc/systemd/system.conf\\\"; sudo cat /etc/systemd/system.conf; sudo systemctl daemon-reload; sudo systemctl restart prosody; sudo systemctl restart jicofo; sudo systemctl restart jitsi-videobridge2; sudo cat /proc/\`sudo cat /var/run/jitsi-videobridge/jitsi-videobridge.pid\`/limits"
	echo "ConfigJitsiCMD: $ConfigJitsiCMD"

	VAR1=$(curl -H "${AUTH}" -sX POST http://$TumblebugServer/tumblebug/ns/$NSID/cmd/mcis/$MCISID/vm/$VMID -H 'Content-Type: application/json' -d @- <<EOF
	{
	"command"        : "${ConfigJitsiCMD}"
	}
EOF
	)
	echo "${VAR1}" | jq . | sed 's/\\n/\n\t - /g'
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
