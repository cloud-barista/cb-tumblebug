#!/bin/bash

#function command_mcis() {
    FILE=../conf.env
    if [ ! -f "$FILE" ]; then
        echo "$FILE does not exist."
        exit
    fi

	echo "[Check jq package (if not, install)]"
	if ! dpkg-query -W -f='${Status}' jq  | grep "ok installed"; then sudo apt install -y jq; fi
	
	source ../conf.env
	AUTH="Authorization: Basic $(echo -n $ApiUsername:$ApiPassword | base64)"

	echo "####################################################################"
	echo "## Command (SSH) to MCIS "
	echo "####################################################################"

	CSP=${1}
	REGION=${2:-1}
	POSTFIX=${3:-developer}
	if [ "${CSP}" == "all" ]; then
		echo "[Test for all CSP regions (AWS, Azure, GCP, Alibaba, ...)]"
		CSP="aws"
		INDEX=0
	elif [ "${CSP}" == "aws" ]; then
		echo "[Test for AWS]"
		INDEX=1
	elif [ "${CSP}" == "azure" ]; then
		echo "[Test for Azure]"
		INDEX=2
	elif [ "${CSP}" == "gcp" ]; then
		echo "[Test for GCP]"
		INDEX=3
	elif [ "${CSP}" == "alibaba" ]; then
		echo "[Test for Alibaba]"
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

	MCISINFO=`curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NS_ID/mcis/${MCISID}?action=status`
	VMARRAY=$(jq -r '.status.vm' <<< "$MCISINFO")
	MASTERIP=$(jq -r '.status.masterIp' <<< "$MCISINFO")
	MASTERVM=$(jq -r '.status.masterVmId' <<< "$MCISINFO")
	
	echo "MASTERIP: $MASTERIP"
	echo "MASTERVM: $MASTERVM"	
	echo "VMARRAY: $VMARRAY"

	IPLIST=""

	for row in $(echo "${VMARRAY}" | jq -r '.[] | @base64'); do
		_jq() {
		echo ${row} | base64 --decode | jq -r ${1}
		}

		IPLIST+=$(_jq '.public_ip')
		IPLIST+=" "
	done

	IPLIST=`echo ${IPLIST}`
	echo "IPLIST: $IPLIST"
	LAUNCHCMD="sudo scope launch $IPLIST"
	#echo $LAUNCHCMD

	echo "[Install Weavescope]"	
	curl -H "${AUTH}" -sX POST http://$TumblebugServer/tumblebug/ns/$NS_ID/cmd/mcis/$MCISID -H 'Content-Type: application/json' -d \
	'{
	"command": "sudo apt-get update > /dev/null;  sudo apt install docker.io -y; sudo curl -L git.io/scope -o /usr/local/bin/scope; sudo chmod a+x /usr/local/bin/scope"
	}' | json_pp 
	echo ""

	echo "[Start Weavescope] master"
	curl -H "${AUTH}" -sX POST http://$TumblebugServer/tumblebug/ns/$NS_ID/cmd/mcis/$MCISID/vm/$MASTERVM -H 'Content-Type: application/json' -d @- <<EOF
	{
	"command"        : "${LAUNCHCMD}"
	}
EOF
	echo ""

	echo "[Start Weavescope] the others"
	curl -H "${AUTH}" -sX POST http://$TumblebugServer/tumblebug/ns/$NS_ID/cmd/mcis/$MCISID -H 'Content-Type: application/json' -d @- <<EOF
	{
	"command"        : "${LAUNCHCMD}"
	}
EOF

	echo ""
	echo "[Access MCIS Weavescope]"
	echo "URL: $MASTERIP:4040/#!/state/{\"topologyId\":\"hosts\"}"
	echo ""	