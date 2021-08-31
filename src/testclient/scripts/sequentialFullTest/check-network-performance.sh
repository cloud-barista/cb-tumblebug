#!/bin/bash

TestSetFile=${4:-../testSet.env}
if [ ! -f "$TestSetFile" ]; then
	echo "$TestSetFile does not exist."
	exit
fi
source $TestSetFile
source ../conf.env

echo "####################################################################"
echo "## Check network performances in MC-Infra (fping, iperf)"
echo "####################################################################"

CSP=${1}
REGION=${2:-1}
POSTFIX=${3:-developer}

source ../common-functions.sh
getCloudIndex $CSP

MCISID=${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX}

if [ "${INDEX}" == "0" ]; then
	# MCISPREFIX=avengers
	MCISID=${MCISPREFIX}-${POSTFIX}
fi

#install jq and puttygen
echo "[Check jq and putty-tools package (if not, install)]"
if ! dpkg-query -W -f='${Status}' jq | grep "ok installed"; then sudo apt install -y jq; fi
if ! dpkg-query -W -f='${Status}' putty-tools | grep "ok installed"; then sudo apt install -y putty-tools; fi

# curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NSID/resources/sshKey/$MCIRID -H 'Content-Type: application/json' | jq '.privateKey' | sed -e 's/\\n/\n/g' -e 's/\"//g' > ./sshkey-tmp/$MCISID.pem
# chmod 600 ./sshkey-tmp/$MCISID.pem
# puttygen ./sshkey-tmp/$MCISID.pem -o ./sshkey-tmp/$MCISID.ppk -O private

echo ""
#echo "[CHECK REMOTE COMMAND BY CB-TB API]"
#echo " This will retrieve verified SSH username"

#./command-mcis.sh -c $CSP -r $REGION -n $POSTFIX -f $TestSetFile

MCISINFO=$(curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NSID/mcis/${MCISID}?action=status)
VMARRAY=$(jq '.status.vm' <<<"$MCISINFO")

# echo "$VMARRAY" | jq ''

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

	#printf ' [VMIP]: %s   [MCISID]: %s   [VMID]: %s\n' "$ip" "$MCISID" "$id"

	#echo -e " ./sshkey-tmp/$KEYFILENAME.pem \n ./sshkey-tmp/$KEYFILENAME.ppk"
	#echo ""
done

IPLIST=""
PRIVIPLIST=""
IPCOMBLIST=""

for row in $(echo "${VMARRAY}" | jq -r '.[] | @base64'); do
	_jq() {
		echo ${row} | base64 --decode | jq -r ${1}
	}

	IPLIST+=$(_jq '.publicIp')
	IPLIST+=" "

	PRIVIPLIST+=$(_jq '.privateIp')
	PRIVIPLIST+=" "

	IPCOMBLIST+=$(_jq '.publicIp')
	IPCOMBLIST+=" "
	IPCOMBLIST+=$(_jq '.privateIp')
	IPCOMBLIST+=" "
done

IPLIST=$(echo ${IPLIST})
echo "IPLIST: $IPLIST"

PRIVIPLIST=$(echo ${PRIVIPLIST})
echo "PRIVIPLIST: $PRIVIPLIST"

IPCOMBLIST=$(echo ${IPCOMBLIST})
echo "IPCOMBLIST: $IPCOMBLIST"

echo ""
echo "[SSH COMMAND EXAMPLE]"
for row in $(echo "${VMARRAY}" | jq -r '.[] | @base64'); do
	_jq() {
		echo ${row} | base64 --decode | jq -r ${1}
	}

	id=$(_jq '.id')
	ip=$(_jq '.publicIp')
	privIp=$(_jq '.privateIp')

	VMINFO=$(curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NSID/mcis/${MCISID}/vm/${id})
	VMKEYID=$(jq -r '.sshKeyId' <<<"$VMINFO")

	KEYINFO=$(curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/${NSID}/resources/sshKey/${VMKEYID})
	USERNAME=$(jq -r '.verifiedUsername' <<<"$KEYINFO")

	# KEYFILENAME="MCIS_${MCISID}_VM_${id}"
	KEYFILENAME="${VMKEYID}"

	echo ""
	printf ' [VMIP]: %s (priv: %s)   [MCISID]: %s   [VMID]: %s\n' "$ip" "$privIp" "$MCISID" "$id"
	printf ' ssh -i ./sshkey-tmp/%s.pem %s@%s -o StrictHostKeyChecking=no\n' "$KEYFILENAME" "$USERNAME" "$ip"
	# ssh -i ./sshkey-tmp/$KEYFILENAME.pem $USERNAME@$ip -o StrictHostKeyChecking=no "fping -e $PRIVIPLIST"
done

echo ""

LAUNCHCMD="sudo apt-get update > /dev/null; sudo apt-get install -y fping > /dev/null; fping -v | grep Version"

echo "[Prepare fping (CMD: $LAUNCHCMD)]"

VAR1=$(curl -H "${AUTH}" -sX POST http://$TumblebugServer/tumblebug/ns/$NSID/cmd/mcis/$MCISID -H 'Content-Type: application/json' -d @- <<EOF
	{
	"command"        : "${LAUNCHCMD}"
	}
EOF
)
# echo "${VAR1}"

for row in $(echo "${VAR1}" | jq '.result_array' | jq -r '.[] | @base64'); do
	_jq() {
		echo ${row} | base64 --decode | jq -r ${1}
	}

	result=$(_jq '.result')
	echo ""

	printf '[fping version] %s' "$result"
done

echo ""

LAUNCHCMD="fping -e $PRIVIPLIST"
#LAUNCHCMD="fping -e $IPCOMBLIST"
# LAUNCHCMD="fping $PRIVIPLIST -q -n -c 10"

VAR1=$(curl -H "${AUTH}" -sX POST http://$TumblebugServer/tumblebug/ns/$NSID/cmd/mcis/$MCISID -H 'Content-Type: application/json' -d @- <<EOF
	{
	"command"        : "${LAUNCHCMD}"
	}
EOF
)

# echo "${VAR1}"

echo ""
echo "[Ping to all nodes in MC-Infra (CMD: $LAUNCHCMD)]"

for row in $(echo "${VAR1}" | jq '.result_array' | jq -r '.[] | @base64'); do
	_jq() {
		echo ${row} | base64 --decode | jq -r ${1}
	}

	vmId=$(_jq '.vmId')
	vmIp=$(_jq '.vmIp')
	result=$(_jq '.result' | sort)
	echo ""

	printf '[%s (%s)] ping to \n%s \n' "$vmId" "$vmIp" "$result"
done

echo "Done!"
