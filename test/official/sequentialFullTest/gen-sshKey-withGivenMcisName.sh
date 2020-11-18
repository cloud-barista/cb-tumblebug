#!/bin/bash

#function get_sshKey() {



    FILE=../conf.env
    if [ ! -f "$FILE" ]; then
        echo "$FILE does not exist."
        exit
    fi

	source ../conf.env
	AUTH="Authorization: Basic $(echo -n $ApiUsername:$ApiPassword | base64)"

	echo "####################################################################"
	echo "## Generate SSH KEY (PEM, PPK)" 
	echo "####################################################################"

	CSP=${1}
	REGION=${2:-1}
	POSTFIX=${3:-developer}
	MCISNAME=${4:-noname}
	if [ "${CSP}" == "all" ]; then
		echo "[Test for all CSP regions (AWS, Azure, GCP, Alibaba, ...)]"
		CSP="aws"
		INDEX=0
	elif [ "${CSP}" == "aws" ]; then
		INDEX=1
	elif [ "${CSP}" == "azure" ]; then
		INDEX=2
	elif [ "${CSP}" == "gcp" ]; then
		INDEX=3
	elif [ "${CSP}" == "alibaba" ]; then
		INDEX=4
	else
		echo "[No acceptable argument was provided (all, aws, azure, gcp, alibaba, ...). Default: Test for AWS]"
		CSP="aws"
		INDEX=1
	fi


	MCISID=${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX}

	if [ "${INDEX}" == "0" ]; then
		MCISPREFIX=avengers
		MCISID=${MCISPREFIX}-${POSTFIX}
	fi

	if [ "${MCISNAME}" != "noname" ]; then
		echo "[MCIS name is given]"
		MCISID=${MCISNAME}
	fi

	#install jq and puttygen
	echo "[Check jq and putty-tools package (if not, install)]"
	if ! dpkg-query -W -f='${Status}' jq  | grep "ok installed"; then sudo apt install -y jq; fi
	if ! dpkg-query -W -f='${Status}' putty-tools  | grep "ok installed"; then sudo apt install -y putty-tools; fi


	curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NS_ID/resources/sshKey/$MCISID -H 'Content-Type: application/json' | jq '.privateKey' | sed -e 's/\\n/\n/g' -e 's/\"//g' > ./sshkey-tmp/$MCISID.pem
	chmod 600 ./sshkey-tmp/$MCISID.pem
	puttygen ./sshkey-tmp/$MCISID.pem -o ./sshkey-tmp/$MCISID.ppk -O private

	MCISINFO=`curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NS_ID/mcis/${MCISID}?action=status`
	MCISINFO=$(jq '.status.vm' <<< "$MCISINFO")

	echo "$MCISINFO" | jq


	echo ""
	echo "[GENERATED PRIVATE KEY (PEM, PPK)]"
	echo -e " ./sshkey-tmp/$MCISID.pem \n ./sshkey-tmp/$MCISID.ppk"
	echo ""

	echo "[MCIS INFO: $MCISID]"
	for k in $(jq -c '.[]' <<< "$MCISINFO"); do
		
		id=$(jq ".id" <<< "$k");
		ip=$(jq ".public_ip" <<< "$k");
		printf ' VMID: %s \t VMIP: %s\n' "$id" "$ip";

	done 

	echo ""
	echo "[SSH USAGE EXAMPLE]"
	for k in $(jq -c '.[]' <<< "$MCISINFO"); do
		
		id=$(jq -r ".id" <<< "$k");
		ip=$(jq -r ".public_ip" <<< "$k");
		user="ubuntu"
		printf ' ssh -i ./sshkey-tmp/%s.pem %s@%s -o StrictHostKeyChecking=no\n' "$MCISID" "$user" "$ip";
		#echo "Use [ssh -i ./sshkey-tmp/$MCISID.pem $user@$ip]"

	done 

	echo ""


#}

#get_sshKey