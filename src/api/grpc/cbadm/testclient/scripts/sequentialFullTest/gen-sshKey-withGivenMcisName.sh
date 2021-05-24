#!/bin/bash

#function get_sshKey() {





	TestSetFile=${5:-../testSet.env}
    
    if [ ! -f "$TestSetFile" ]; then
        echo "$TestSetFile does not exist."
        exit
    fi
	source $TestSetFile
    source ../conf.env
	
	echo "####################################################################"
	echo "## Generate SSH KEY (PEM, PPK)" 
	echo "####################################################################"

	CSP=${1}
	REGION=${2:-1}
	POSTFIX=${3:-developer}
	MCISNAME=${4:-noname}

	source ../common-functions.sh
	getCloudIndex $CSP


	MCISID=${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX}

	if [ "${INDEX}" == "0" ]; then
		# MCISPREFIX=avengers
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


	$CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/cbadm keypair get --config $CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/grpc_conf.yaml -o json --ns $NSID --id $MCISID | jq '.privateKey' | sed -e 's/\\n/\n/g' -e 's/\"//g' > ./sshkey-tmp/$MCISID.pem
	chmod 600 ./sshkey-tmp/$MCISID.pem
	puttygen ./sshkey-tmp/$MCISID.pem -o ./sshkey-tmp/$MCISID.ppk -O private

	MCISINFO=`$CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/cbadm mcis status --config $CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/grpc_conf.yaml -o json --ns $NSID --mcis ${MCISID}`
	MCISINFO=$(jq '.status.vm' <<< "$MCISINFO")

	echo "$MCISINFO" | jq


	echo ""
	echo "[GENERATED PRIVATE KEY (PEM, PPK)]"
	echo -e " ./sshkey-tmp/$MCISID.pem \n ./sshkey-tmp/$MCISID.ppk"
	echo ""

	echo "[MCIS INFO: $MCISID]"
	for row in $(jq -r '.[] | @base64' <<< "$MCISINFO"); do
		
		k=$(echo ${row} | base64 --decode | jq -r .)
		
		id=$(jq ".id" <<< "$k");
		ip=$(jq ".publicIp" <<< "$k");
		printf ' VMID: %s \t VMIP: %s\n' "$id" "$ip";

	done 

	echo ""
	echo "[SSH USAGE EXAMPLE]"
	for row in $(jq -r '.[] | @base64' <<< "$MCISINFO"); do
		
		k=$(echo ${row} | base64 --decode | jq -r .)
		
		id=$(jq -r ".id" <<< "$k");
		ip=$(jq -r ".publicIp" <<< "$k");
		user="ubuntu"
		printf ' ssh -i ./sshkey-tmp/%s.pem %s@%s -o StrictHostKeyChecking=no\n' "$MCISID" "$user" "$ip";
		#echo "Use [ssh -i ./sshkey-tmp/$MCISID.pem $user@$ip]"

	done 

	echo ""


#}

#get_sshKey