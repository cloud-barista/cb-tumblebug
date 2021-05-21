#!/bin/bash

#function get_cloud() {


    FILE=../credentials.conf
    if [ ! -f "$FILE" ]; then
        echo "$FILE does not exist."
        exit
    fi


    TestSetFile=${4:-../testSet.env}
    if [ ! -f "$TestSetFile" ]; then
        echo "$TestSetFile does not exist."
        exit
    fi
	source $TestSetFile
    source ../conf.env
    source ../credentials.conf
    
    echo "####################################################################"
    echo "## 0. Get Cloud Connction Config"
    echo "####################################################################"

    CSP=${1}
    REGION=${2:-1}
    POSTFIX=${3:-developer}
    
	source ../common-functions.sh
	getCloudIndex $CSP

    RESTSERVER=localhost

    # for Cloud Connection Config Info
    curl -H "${AUTH}" -sX GET http://$SpiderServer/spider/connectionconfig/${CONN_CONFIG[$INDEX,$REGION]} | jq ''
    echo ""


    # for Cloud Region Info
    curl -H "${AUTH}" -sX GET http://$SpiderServer/spider/region/${RegionName[$INDEX,$REGION]} | jq ''
    echo ""


    # for Cloud Credential Info
    curl -H "${AUTH}" -sX GET http://$SpiderServer/spider/credential/${CredentialName[$INDEX]} | jq ''
    echo ""

    
    # for Cloud Driver Info
    curl -H "${AUTH}" -sX GET http://$SpiderServer/spider/driver/${DriverName[$INDEX]} | jq ''
    echo ""
#}

#get_cloud
