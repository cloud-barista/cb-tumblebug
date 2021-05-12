#!/bin/bash

TestSetFile=${4:-../testSet.env}
if [ ! -f "$TestSetFile" ]; then
	echo "$TestSetFile does not exist."
	exit
fi
source $TestSetFile
source ../conf.env

echo "####################################################################"
echo "## Config Ansible environment (generate host file)"
echo "####################################################################"

CSP=${1}
REGION=${2:-1}
POSTFIX=${3:-developer}

source ../common-functions.sh
getCloudIndex $CSP

MCISID=${CONN_CONFIG[$INDEX, $REGION]}-${POSTFIX}

if [ "${INDEX}" == "0" ]; then
	# MCISPREFIX=avengers
	MCISID=${MCISPREFIX}-${POSTFIX}
fi

#install jq and puttygen
echo "[Check jq and putty-tools package (if not, install)]"
if ! dpkg-query -W -f='${Status}' jq | grep "ok installed"; then sudo apt install -y jq; fi
if ! dpkg-query -W -f='${Status}' putty-tools | grep "ok installed"; then sudo apt install -y putty-tools; fi

echo "[Check Ansible (if not, Exit)]"

printf '[Command to install Ansible]\n 1. apt install python-pip\n 2. pip install ansible\n 3. ansible -h\n 4. ansible localhost -m ping'
# apt install python-pip
# pip install ansible
# ansible -h
# ansible localhost -m ping
if ! dpkg-query -W -f='${Status}' ansible | grep "ok installed"; then exit; fi

# curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NSID/resources/sshKey/$MCIRID -H 'Content-Type: application/json' | jq '.privateKey' | sed -e 's/\\n/\n/g' -e 's/\"//g' > ./sshkey-tmp/$MCISID.pem
# chmod 600 ./sshkey-tmp/$MCISID.pem
# puttygen ./sshkey-tmp/$MCISID.pem -o ./sshkey-tmp/$MCISID.ppk -O private

echo ""
echo "[CHECK REMOTE COMMAND BY CB-TB API]"
echo " This will retrieve verified SSH username"

./command-mcis.sh $CSP $REGION $POSTFIX $TestSetFile

MCISINFO=$(curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NSID/mcis/${MCISID}?action=status)
VMARRAY=$(jq '.status.vm' <<<"$MCISINFO")

echo "$VMARRAY" | jq ''

echo ""
echo "[GENERATED PRIVATE KEY (PEM, PPK)]"
# echo -e " ./sshkey-tmp/$MCISID.pem \n ./sshkey-tmp/$MCISID.ppk"
echo ""

echo "[MCIS INFO: $MCISID]"
for row in $(echo "${VMARRAY}" | jq -r '.[] | @base64'); do
	_jq() {
		echo ${row} | base64 --decode | jq -r ${1}
	}

	id=$(_jq '.id')
	ip=$(_jq '.publicIp')

	VMINFO=$(curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NSID/mcis/${MCISID}/vm/${id})
	VMKEYID=$(jq -r '.sshKeyId' <<<"$VMINFO")

	# KEYFILENAME="MCIS_${MCISID}_VM_${id}"
	KEYFILENAME="${VMKEYID}"

	curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NSID/resources/sshKey/$VMKEYID -H 'Content-Type: application/json' | jq '.privateKey' | sed -e 's/\\n/\n/g' -e 's/\"//g' >./sshkey-tmp/$KEYFILENAME.pem
	chmod 600 ./sshkey-tmp/$KEYFILENAME.pem
	puttygen ./sshkey-tmp/$KEYFILENAME.pem -o ./sshkey-tmp/$KEYFILENAME.ppk -O private

	printf ' [VMIP]: %s   [MCISID]: %s   [VMID]: %s\n' "$ip" "MCISID" "$id"

	echo -e " ./sshkey-tmp/$KEYFILENAME.pem \n ./sshkey-tmp/$KEYFILENAME.ppk"
	echo ""
done

echo ""
echo "[Configure Ansible Host File based on MCIS Info]"

HostFileName="${MCISID}-host"
echo "[mcis_hosts]" >./ansibleAutoConf/${HostFileName}

for row in $(echo "${VMARRAY}" | jq -r '.[] | @base64'); do
	_jq() {
		echo ${row} | base64 --decode | jq -r ${1}
	}

	id=$(_jq '.id')
	ip=$(_jq '.publicIp')

	VMINFO=$(curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NSID/mcis/${MCISID}/vm/${id})
	VMKEYID=$(jq -r '.sshKeyId' <<<"$VMINFO")

	KEYINFO=$(curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/${NSID}/resources/sshKey/${VMKEYID})
	USERNAME=$(jq -r '.verifiedUsername' <<<"$KEYINFO")


	KEYFILENAME="${VMKEYID}"

	echo ""

	printf ' [VMIP]: %s   [MCISID]: %s   [VMID]: %s\n' "$ip" "MCISID" "$id"
	printf ' ssh -i ./sshkey-tmp/%s.pem %s@%s -o StrictHostKeyChecking=no\n' "$KEYFILENAME" "$USERNAME" "$ip"

	echo "Add ansible hosts to ./ansibleAutoConf/${HostFileName}"
	echo "- $ip ansible_ssh_port=22 ansible_user=$USERNAME ansible_ssh_private_key_file=./sshkey-tmp/$KEYFILENAME.pem ansible_ssh_common_args=\"-o StrictHostKeyChecking=no\""
	echo "$ip ansible_ssh_port=22 ansible_user=$USERNAME ansible_ssh_private_key_file=./sshkey-tmp/$KEYFILENAME.pem ansible_ssh_common_args=\"-o StrictHostKeyChecking=no\"" >>./ansibleAutoConf/${HostFileName}

	echo ""
	echo "[You can use Asible in ./ansibleAutoConf/]"
	echo " Ex) ansible-playbook ./ansibleAutoConf/helloworld.yml -i ./ansibleAutoConf/${HostFileName}"

done

echo ""
