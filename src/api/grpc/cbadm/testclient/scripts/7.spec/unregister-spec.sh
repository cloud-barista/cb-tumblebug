#!/bin/bash

#function unregister_spec() {


	TestSetFile=${4:-../testSet.env}
    if [ ! -f "$TestSetFile" ]; then
        echo "$TestSetFile does not exist."
        exit
    fi
	source $TestSetFile
    source ../conf.env
	
	echo "####################################################################"
	echo "## 7. spec: Unregister"
	echo "####################################################################"

	CSP=${1}
	REGION=${2:-1}
	POSTFIX=${3:-developer}

	source ../common-functions.sh
	getCloudIndex $CSP

	$CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/cbadm spec delete --config $CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/grpc_conf.yaml -o json --ns $NSID --id ${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX} --force false | jq ''

#}

#unregister_spec
