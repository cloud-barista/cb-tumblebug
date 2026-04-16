#!/bin/bash

echo "####################################################################"
echo "## Copy a local file to all VMs in the Infra (-x source-file-path / -y destination-file-path)"
echo "####################################################################"

source ../init.sh

SOURCEPATH=$OPTION01
DESTPATH=$OPTION02

echo ""

InfraINFO=$(curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NSID/infra/${InfraID})
VMARRAY=$(jq '.vm' <<<"$InfraINFO")

echo ""
echo "[Infra INFO: $InfraID]"

for rowi in $(echo "${VMARRAY}" | jq -r '.[] | @base64'); do
	{
		_jq() {
			echo ${rowi} | base64 --decode | jq -r ${1}
		}

		publicIP=$(_jq '.publicIP')
		vNetId=$(_jq '.vNetId')

		VMKEYID=$(_jq '.sshKeyId')
		KEYINFO=$(curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/${NSID}/resources/sshKey/${VMKEYID})
		USERNAME=$(jq -r '.verifiedUsername' <<<"$KEYINFO")
		KEYFILENAME="${VMKEYID}"


		VAR1=$(scp -o StrictHostKeyChecking=no -i ./sshkey-tmp/$KEYFILENAME.pem ${SOURCEPATH} $USERNAME@$publicIP:${DESTPATH})
		echo "${VAR1}" 

	} &
done
wait

CMD="ls ${DESTPATH}"
VAR1=$(
	curl -H "${AUTH}" -sX POST http://$TumblebugServer/tumblebug/ns/$NSID/cmd/infra/$InfraID -H 'Content-Type: application/json' -d @- <<EOF
	{
	"command"        : "[${CMD}]"
	}
EOF
)
echo "${VAR1}" | jq . | sed 's/\\n/\n/g'
