#!/bin/bash

#function list_cloud() {


    TestSetFile=${4:-../testSet.env}
    if [ ! -f "$TestSetFile" ]; then
        echo "$TestSetFile does not exist."
        exit
    fi
	source $TestSetFile
    source ../conf.env
    #source ../credentials.conf
    
    echo "####################################################################"
    echo "## 0. List Cloud Connction Config(s)"
    echo "####################################################################"


    #INDEX=${1}

    RESTSERVER=localhost

    # for Cloud Connection Config Info
    curl -H "${AUTH}" -sX GET http://$SpiderServer/spider/connectionconfig | jq ''
    echo ""


    # for Cloud Region Info
    curl -H "${AUTH}" -sX GET http://$SpiderServer/spider/region | jq ''
    echo ""


    # for Cloud Credential Info
    curl -H "${AUTH}" -sX GET http://$SpiderServer/spider/credential | jq ''
    echo ""
    
    
    # for Cloud Driver Info
    curl -H "${AUTH}" -sX GET http://$SpiderServer/spider/driver | jq ''
    echo ""
#}

#list_cloud
