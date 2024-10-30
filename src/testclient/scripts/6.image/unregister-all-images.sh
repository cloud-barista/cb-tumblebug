#!/bin/bash

#function unregister_image() {

    SECONDS=0

	# TestSetFile=${4:-../testSet.env}
    # if [ ! -f "$TestSetFile" ]; then
    #     echo "$TestSetFile does not exist."
    #     exit
    # fi
	# source $TestSetFile
    source ../conf.env
	
	echo "####################################################################"
	echo "## 6. image: Unregister"
	echo "####################################################################"

	# CSP=${1}
	# REGION=${2:-1}
	# POSTFIX=${3:-developer}

	source ../common-functions.sh
	# getCloudIndex $CSP

	curl -H "${AUTH}" -sX DELETE http://$TumblebugServer/tumblebug/ns/$NSID/resources/image | jq '.'

    printElapsed $@
#}

#unregister_image
