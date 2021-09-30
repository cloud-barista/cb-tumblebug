#!/bin/bash

#function deploy_nginx_to_mcis() {


	TestSetFile=${4:-../testSet.env}
    if [ ! -f "$TestSetFile" ]; then
        echo "$TestSetFile does not exist."
        exit
    fi
	source $TestSetFile
    source ../conf.env
	
	echo "####################################################################"
	echo "## Command (SSH) to MCIS "
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

	$CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/cbadm mcis command --config $CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/grpc_conf.yaml -i json -o json -d \
	"{
		\"nsId\":  \"${NSID}\",
		\"mcisId\": \"${MCISID}\",
		\"cmd\": {
			\"command\": \"wget https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/scripts/setcbtb.sh -O ~/setcbtb.sh; chmod +x ~/setcbtb.sh; ~/setcbtb.sh\"
		}
	}" | jq '' #|| return 1
#}

#deploy_nginx_to_mcis