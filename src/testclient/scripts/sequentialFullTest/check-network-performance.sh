#!/bin/bash

echo "####################################################################"
echo "## Check network performances in MC-Infra (fping, iperf)"
echo "####################################################################"

source ../init.sh

echo ""

MCISINFO=$(curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NSID/mcis/${MCISID}?action=status)
VMARRAY=$(jq '.status.vm' <<<"$MCISINFO")

echo ""
echo "[GENERATED PRIVATE KEY (PEM, PPK)]"

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

done

VMLIST=""
IPLIST=""
PRIVIPLIST=""
IPCOMBLIST=""

for row in $(echo "${VMARRAY}" | jq -r '.[] | @base64'); do
	_jq() {
		echo ${row} | base64 --decode | jq -r ${1}
	}

	VMLIST+=$(_jq '.id')
	VMLIST+=" "

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

LAUNCHCMD="sudo apt-get update > /dev/null; sudo apt-get install -y iputils-ping fping > /dev/null; fping -v | grep Version"

echo "[Prepare fping (CMD: $LAUNCHCMD)]"

VAR1=$(
	curl -H "${AUTH}" -sX POST http://$TumblebugServer/tumblebug/ns/$NSID/cmd/mcis/$MCISID -H 'Content-Type: application/json' -d @- <<EOF
	{
	"command"        : "${LAUNCHCMD}"
	}
EOF
)
# echo "${VAR1}"

for row in $(echo "${VAR1}" | jq '.resultArray' | jq -r '.[] | @base64'); do
	_jq() {
		echo ${row} | base64 --decode | jq -r ${1}
	}

	result=$(_jq '.result')
	echo ""

	printf '[fping version] %s' "$result"
done

echo ""

#LAUNCHCMD="fping -e $PRIVIPLIST"
LAUNCHCMD="fping -e $IPLIST"
#LAUNCHCMD="fping -e $IPCOMBLIST"
# LAUNCHCMD="fping $PRIVIPLIST -q -n -c 10"

VAR1=$(
	curl -H "${AUTH}" -sX POST http://$TumblebugServer/tumblebug/ns/$NSID/cmd/mcis/$MCISID -H 'Content-Type: application/json' -d @- <<EOF
	{
	"command"        : "${LAUNCHCMD}"
	}
EOF
)

# echo "${VAR1}"

echo ""
echo "[Ping to all nodes in MC-Infra (CMD: $LAUNCHCMD)]"

for row in $(echo "${VAR1}" | jq '.resultArray' | jq -r '.[] | @base64'); do
	_jq() {
		echo ${row} | base64 --decode | jq -r ${1}
	}

	vmId=$(_jq '.vmId')
	vmIp=$(_jq '.vmIp')
	result=$(_jq '.result' | sort)
	echo ""

	printf '[%s (%s)] ping to \n%s \n' "$vmId" "$vmIp" "$result"
done

PublicIPListArray=($IPLIST)
PrivateIPListArray=($PRIVIPLIST)
VMIDListArray=($VMLIST)

echo ""
printf "%-15s" "PublicIP:"
for i in "${PublicIPListArray[@]}"; do
	printf "%15s" $i
done
echo ""
printf "%-15s" "PrivateIP:"
for i in "${PrivateIPListArray[@]}"; do
	printf "%15s" $i
done
echo ""
printf "%-15s" "VMID:"
for i in "${VMIDListArray[@]}"; do
	printf "%15s" $i
done
echo ""
printf "%-15s" "-------"
for i in "${VMIDListArray[@]}"; do
	printf "%15s" "-------"
done
echo ""
# for i in "${VMIDListArray[@]}"; do

# 	printf "%-15s" $i
# 	for k in "${PublicIPListArray[@]}"; do
# 		LAUNCHCMD="ping -c 3 -W 1 $k | tail -1 " # | awk '{print $4}' | cut -d '/' -f 2

# 		VAR1=$(
# 			curl -H "${AUTH}" -sX POST http://$TumblebugServer/tumblebug/ns/$NSID/cmd/mcis/$MCISID/vm/$i -H 'Content-Type: application/json' -d @- <<EOF
# 	{
# 	"command"        : "${LAUNCHCMD}"
# 	}
# EOF
# 		)

# 		VAR1=$(echo $VAR1 | awk '{print $4}' | cut -d '/' -f 2)
# 		printf "%15s" $VAR1
# 	done
# 	echo ""
# done

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

	#echo ""
	#printf ' [VMIP]: %s (priv: %s)   [MCISID]: %s   [VMID]: %s\n' "$ip" "$privIp" "$MCISID" "$id"
	#printf ' ssh -i ./sshkey-tmp/%s.pem %s@%s -o StrictHostKeyChecking=no\n' "$KEYFILENAME" "$USERNAME" "$ip"
	# ssh -i ./sshkey-tmp/$KEYFILENAME.pem $USERNAME@$ip -o StrictHostKeyChecking=no "fping -e $PRIVIPLIST"
	printf "%-15s" $id
	for k in "${PublicIPListArray[@]}"; do
		LAUNCHCMD="ping -c 3 -W 1 $k | tail -1 " # | awk '{print $4}' | cut -d '/' -f 2

		VAR1=$(ssh -i ./sshkey-tmp/$KEYFILENAME.pem $USERNAME@$ip -o StrictHostKeyChecking=no "$LAUNCHCMD")

		VAR1=$(echo $VAR1 | awk '{print $4}' | cut -d '/' -f 2)
		printf "%15s" $VAR1
	done
	echo ""
done

