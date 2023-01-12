#!/bin/bash

# Execute with sudo
# Required Parameters: IP, DNS, EMAIL
echo "[Start Jitsi Video Conference Server]"

SECONDS=0

IP=${1}
DNS=${2}
EMAIL=${3}

echo "(1)IP= $IP / (2)DNS= $DNS / (3)EMAIL= $EMAIL"

if [ -z "$IP" ]; then
	echo "[Warning] Provide public IP of the Host "
	exit
fi

if [ -z "$EMAIL" ]; then
	echo "[Warning] Provide your E-MAIL (ex: xxx@cloudbarista.org)"
	echo "E-MAIL address will be used to issue a Certificate (https) for JITSI"
	exit
fi

if [ -z "$DNS" ]; then
	echo "[Warning] Provide your PublicDNS-RecordName (ex: xxx.cloud-barista.org)"
	echo "PublicDNS-RecordName will be access point for JITSI Server"
	exit
fi

sudo -- sh -c "echo $IP $DNS >> /etc/hosts"
sudo hostnamectl set-hostname $DNS; hostname -f

echo "[Remove (if) existing packages to initialize jitsi]"
sudo apt purge jigasi jitsi-meet jitsi-meet-web-config jitsi-meet-prosody jitsi-meet-turnserver jitsi-meet-web jicofo jitsi-videobridge2 -y &> /dev/null

echo "[Install Jitsi]"
curl https://prosody.im/files/prosody-debian-packages.key | sudo sh -c 'gpg --dearmor > /usr/share/keyrings/prosody-keyring.gpg'
echo 'deb [signed-by=/usr/share/keyrings/prosody-keyring.gpg] http://packages.prosody.im/debian jammy main' | sudo tee -a /etc/apt/sources.list.d/prosody.list &> /dev/null

curl https://download.jitsi.org/jitsi-key.gpg.key | sudo sh -c 'gpg --dearmor > /usr/share/keyrings/jitsi-keyring.gpg'
echo 'deb [signed-by=/usr/share/keyrings/jitsi-keyring.gpg] https://download.jitsi.org stable/' | sudo tee /etc/apt/sources.list.d/jitsi-stable.list &> /dev/null

sudo apt update > /dev/null

sudo apt remove needrestart -y &> /dev/null

echo "jitsi-videobridge2  jitsi-videobridge/jvb-hostname  string  $DNS" | sudo debconf-set-selections
sudo DEBIAN_FRONTEND=noninteractive apt install jitsi-meet -y > /dev/null

echo "[letsencrypt-certificate (will need actual DNS record)]"
sudo apt install certbot -y &> /dev/null
sudo echo "$EMAIL" | sudo DEBIAN_FRONTEND=noninteractive /usr/share/jitsi-meet/scripts/install-letsencrypt-cert.sh > /dev/null

echo "[Config Jitsi]"
sudo -- sh -c "echo DefaultLimitNOFILE=65000 >> /etc/systemd/system.conf"
sudo -- sh -c "echo DefaultLimitNPROC=65000 >> /etc/systemd/system.conf"
sudo -- sh -c "echo DefaultTasksMax=65000 >> /etc/systemd/system.conf"

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

sudo cat /etc/systemd/system.conf > /dev/null
sudo systemctl daemon-reload
sudo systemctl restart prosody
sudo systemctl restart jicofo
sudo systemctl restart jitsi-videobridge2
sudo cat /proc/`sudo cat /var/run/jitsi-videobridge/jitsi-videobridge.pid`/limits

echo "Done! elapsed time: $SECONDS"

PID=$(ps -ef | grep jitsi | awk '{print $2}')

echo ""
echo "[Start Jitsi: complete]"
echo "PID=$PID"
echo "Host IP: $IP"
echo "Access DNS: $DNS (by a Web browser)"

