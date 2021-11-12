#!/bin/bash

echo "####################################################################"
echo "## Generate SSH KEY (PEM, PPK)"
echo "####################################################################"

source ../init.sh

if [ "${INDEX}" == "0" ]; then
	# MCISPREFIX=avengers
	MCISID=${POSTFIX}
fi

# curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NSID/resources/sshKey/$MCIRID -H 'Content-Type: application/json' | jq '.privateKey' | sed -e 's/\\n/\n/g' -e 's/\"//g' > ./sshkey-tmp/$MCISID.pem
# chmod 600 ./sshkey-tmp/$MCISID.pem
# puttygen ./sshkey-tmp/$MCISID.pem -o ./sshkey-tmp/$MCISID.ppk -O private

echo ""
echo "[CHECK REMOTE COMMAND BY CB-TB API]"
echo " This will retrieve verified SSH username"

./command-mcis.sh -n $POSTFIX -f $TestSetFile

MCISINFO=$(curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NSID/mcis/${MCISID})
VMARRAY=$(jq '.vm' <<<"$MCISINFO")

echo "$VMARRAY" | jq ''

echo ""
echo "[GENERATED PRIVATE KEY (PEM, PPK)]"
# echo -e " ./sshkey-tmp/$MCISID.pem \n ./sshkey-tmp/$MCISID.ppk"
echo ""


echo "[MCIS INFO: $MCISID]"
for row in $(echo "${VMARRAY}" | jq -r '.[] | @base64'); do
	{
		_jq() {
			echo ${row} | base64 --decode | jq -r ${1}
		}

		id=$(_jq '.id')
		ip=$(_jq '.publicIP')]
		VMKEYID=$(_jq '.sshKeyId')

		# KEYFILENAME="MCIS_${MCISID}_VM_${id}"
		KEYFILENAME="${VMKEYID}"

		curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NSID/resources/sshKey/$VMKEYID -H 'Content-Type: application/json' | jq '.privateKey' | sed -e 's/\\n/\n/g' -e 's/\"//g' >./sshkey-tmp/$KEYFILENAME.pem
		chmod 600 ./sshkey-tmp/$KEYFILENAME.pem
		puttygen ./sshkey-tmp/$KEYFILENAME.pem -o ./sshkey-tmp/$KEYFILENAME.ppk -O private

		echo " PEM: ./sshkey-tmp/$KEYFILENAME.pem  PPK: ./sshkey-tmp/$KEYFILENAME.ppk"
	} &
done
wait


echo ""
echo "[SSH COMMAND EXAMPLE]"
for row in $(echo "${VMARRAY}" | jq -r '.[] | @base64'); do
	{
		_jq() {
			echo ${row} | base64 --decode | jq -r ${1}
		}

		id=$(_jq '.id')
		ip=$(_jq '.publicIP')
		privIp=$(_jq '.privateIP')
		VMKEYID=$(_jq '.sshKeyId')

		KEYINFO=$(curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/${NSID}/resources/sshKey/${VMKEYID})
		USERNAME=$(jq -r '.verifiedUsername' <<<"$KEYINFO")

		# KEYFILENAME="MCIS_${MCISID}_VM_${id}"
		KEYFILENAME="${VMKEYID}"

		echo ""
		# USERNAME="ubuntu"
		printf ' [VMIP]: %s (priv: %s)   [MCISID]: %s   [VMID]: %s\n ssh -i ./sshkey-tmp/%s.pem %s@%s -o StrictHostKeyChecking=no\n' "$ip" "$privIp" "$MCISID" "$id" "$KEYFILENAME" "$USERNAME" "$ip"
	} &
done
wait

echo ""
