#!/bin/bash

echo "####################################################################"
echo "## Config Ansible environment (generate host file)"
echo "####################################################################"

SECONDS=0

source ../init.sh

if [ "${INDEX}" == "0" ]; then
	MCIID=${POSTFIX}
fi


echo "[Check Ansible (if not, Exit)]"

printf '[Command to install Ansible]\n 1. apt install python-pip\n 2. pip install ansible\n 3. ansible -h\n 4. ansible localhost -m ping'
# apt install python-pip
# pip install ansible
# ansible -h
# ansible localhost -m ping
if ! dpkg-query -W -f='${Status}' ansible | grep "ok installed"; then exit; fi

# curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NSID/resources/sshKey/$ResourceID -H 'Content-Type: application/json' | jq '.privateKey' | sed -e 's/\\n/\n/g' -e 's/\"//g' > ./sshkey-tmp/$MCIID.pem
# chmod 600 ./sshkey-tmp/$MCIID.pem
# puttygen ./sshkey-tmp/$MCIID.pem -o ./sshkey-tmp/$MCIID.ppk -O private

echo ""
echo "[CHECK REMOTE COMMAND BY CB-TB API]"
echo " This will retrieve verified SSH username"

./command-mci.sh -n $POSTFIX -f $TestSetFile

MCIINFO=$(curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NSID/mci/${MCIID}?option=status)
VMARRAY=$(jq '.status.vm' <<<"$MCIINFO")

echo "$VMARRAY" | jq '.'

echo ""
echo "[GENERATED PRIVATE KEY (PEM, PPK)]"
# echo -e " ./sshkey-tmp/$MCIID.pem \n ./sshkey-tmp/$MCIID.ppk"
echo ""

echo "[MCI INFO: $MCIID]"
for row in $(echo "${VMARRAY}" | jq -r '.[] | @base64'); do
	_jq() {
		echo ${row} | base64 --decode | jq -r ${1}
	}

	id=$(_jq '.id')
	ip=$(_jq '.publicIp')

	VMINFO=$(curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NSID/mci/${MCIID}/vm/${id})
	VMKEYID=$(jq -r '.sshKeyId' <<<"$VMINFO")

	# KEYFILENAME="MCI_${MCIID}_VM_${id}"
	KEYFILENAME="${VMKEYID}"

	curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NSID/resources/sshKey/$VMKEYID -H 'Content-Type: application/json' | jq '.privateKey' | sed -e 's/\\n/\n/g' -e 's/\"//g' >./sshkey-tmp/$KEYFILENAME.pem
	chmod 600 ./sshkey-tmp/$KEYFILENAME.pem
	puttygen ./sshkey-tmp/$KEYFILENAME.pem -o ./sshkey-tmp/$KEYFILENAME.ppk -O private

	printf ' [VMIP]: %s   [MCIID]: %s   [VMID]: %s\n' "$ip" "MCIID" "$id"

	echo -e " ./sshkey-tmp/$KEYFILENAME.pem \n ./sshkey-tmp/$KEYFILENAME.ppk"
	echo ""
done

echo ""
echo "[Configure Ansible Host File based on MCI Info]"

HostFileName="${MCIID}-host"
echo "[mci_hosts]" >./ansibleAutoConf/${HostFileName}

for row in $(echo "${VMARRAY}" | jq -r '.[] | @base64'); do
	_jq() {
		echo ${row} | base64 --decode | jq -r ${1}
	}

	id=$(_jq '.id')
	ip=$(_jq '.publicIp')

	VMINFO=$(curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NSID/mci/${MCIID}/vm/${id})
	VMKEYID=$(jq -r '.sshKeyId' <<<"$VMINFO")

	KEYINFO=$(curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/${NSID}/resources/sshKey/${VMKEYID})
	USERNAME=$(jq -r '.verifiedUsername' <<<"$KEYINFO")


	KEYFILENAME="${VMKEYID}"

	echo ""

	printf ' [VMIP]: %s   [MCIID]: %s   [VMID]: %s\n' "$ip" "MCIID" "$id"
	printf ' ssh -i ./sshkey-tmp/%s.pem %s@%s -o StrictHostKeyChecking=no\n' "$KEYFILENAME" "$USERNAME" "$ip"

	echo "Add ansible hosts to ./ansibleAutoConf/${HostFileName}"
	echo "- $ip ansible_ssh_port=22 ansible_user=$USERNAME ansible_ssh_private_key_file=./sshkey-tmp/$KEYFILENAME.pem ansible_ssh_common_args=\"-o StrictHostKeyChecking=no\""
	echo "$ip ansible_ssh_port=22 ansible_user=$USERNAME ansible_ssh_private_key_file=./sshkey-tmp/$KEYFILENAME.pem ansible_ssh_common_args=\"-o StrictHostKeyChecking=no\"" >>./ansibleAutoConf/${HostFileName}

	echo ""
	echo "[You can use Asible in ./ansibleAutoConf/]"
	echo " Ex) ansible-playbook ./ansibleAutoConf/helloworld.yml -i ./ansibleAutoConf/${HostFileName}"

done

echo ""
