#!/bin/bash

echo "####################################################################"
echo "## Set Local MCI DNS (/etc/hosts of each VM in MC-Infra)"
echo "####################################################################"

source ../init.sh

echo ""

MCIINFO=$(curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NSID/mci/${MCIID})
VMARRAY=$(jq '.vm' <<<"$MCIINFO")

echo ""
echo "[GENERATED PRIVATE KEY (PEM, PPK) first]"

echo ""

echo "[MCI INFO: $MCIID]"
# for row in $(echo "${VMARRAY}" | jq -r '.[] | @base64'); do
# 	_jq() {
# 		echo ${row} | base64 --decode | jq -r ${1}
# 	}

# 	VMKEYID=$(jq '.sshKeyId')
# 	KEYFILENAME="${VMKEYID}"

# 	curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NSID/resources/sshKey/$VMKEYID -H 'Content-Type: application/json' | jq '.privateKey' | sed -e 's/\\n/\n/g' -e 's/\"//g' >./sshkey-tmp/$KEYFILENAME.pem
# 	chmod 600 ./sshkey-tmp/$KEYFILENAME.pem
# 	puttygen ./sshkey-tmp/$KEYFILENAME.pem -o ./sshkey-tmp/$KEYFILENAME.ppk -O private
# done

for rowi in $(echo "${VMARRAY}" | jq -r '.[] | @base64'); do
	{
		_jq() {
			echo ${rowi} | base64 --decode | jq -r ${1}
		}

		VMID=$(_jq '.id')
		publicIP=$(_jq '.publicIP')
		vNetId=$(_jq '.vNetId')

		echo "VMID: $VMID"

		VMKEYID=$(_jq '.sshKeyId')
		KEYINFO=$(curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/${NSID}/resources/sshKey/${VMKEYID})
		USERNAME=$(jq -r '.verifiedUsername' <<<"$KEYINFO")
		KEYFILENAME="${VMKEYID}"

		for rowj in $(echo "${VMARRAY}" | jq -r '.[] | @base64'); do
			_jq() {
				echo ${rowj} | base64 --decode | jq -r ${1}
			}

			Bid=$(_jq '.id')
			BpublicIP=$(_jq '.publicIP')
			BprivateIP=$(_jq '.privateIP')
			BvNetId=$(_jq '.vNetId')

			CommandToAddHosts=""

			if [ "${VMID}" == "${Bid}" ]; then
				CommandToAddHosts="sudo sed -i '2i127.0.0.1 ${Bid}' /etc/hosts"
			elif [ "${vNetId}" == "${BvNetId}" ]; then
				CommandToAddHosts="sudo sed -i '2i${BprivateIP} ${Bid}' /etc/hosts"
			else
				CommandToAddHosts="sudo sed -i '2i${BpublicIP} ${Bid}' /etc/hosts"
			fi

			echo "${CommandToAddHosts}"

			VAR1=$(ssh -i ./sshkey-tmp/$KEYFILENAME.pem $USERNAME@$publicIP -o StrictHostKeyChecking=no "$CommandToAddHosts")
			echo "${VAR1}" | jq '.'

		done
	} &
done
wait

CMD="cat /etc/hosts"
VAR1=$(
	curl -H "${AUTH}" -sX POST http://$TumblebugServer/tumblebug/ns/$NSID/cmd/mci/$MCIID -H 'Content-Type: application/json' -d @- <<EOF
	{
	"command"        : "[${CMD}]"
	}
EOF
)
echo "${VAR1}" | jq . | sed 's/\\n/\n/g'
