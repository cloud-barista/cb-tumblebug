#!/bin/bash

function CallSpider() {
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
}

#function get_cloud() {

    echo "####################################################################"
    echo "## 0. Get Cloud Connction Config"
    echo "####################################################################"

    source ../init.sh

    echo "AUTH: $AUTH"
    echo "TumblebugServer: $TumblebugServer"
    echo "NSID: $NSID"
    echo "INDEX: $INDEX"
    echo "REGION: $REGION"
    echo "{CONN_CONFIG[$INDEX,$REGION]}: ${CONN_CONFIG[$INDEX,$REGION]}"
    echo "POSTFIX: $POSTFIX"
    echo ""

    if [ "${INDEX}" == "0" ]; then
        echo "[Parallel execution for all CSP regions]"
        INDEXX=${NumCSP}
        for ((cspi = 1; cspi <= INDEXX; cspi++)); do
            INDEXY=${NumRegion[$cspi]}
            CSP=${CSPType[$cspi]}
            echo "[$cspi] $CSP details"
            for ((cspj = 1; cspj <= INDEXY; cspj++)); do
                echo "[$cspi,$cspj] ${RegionName[$cspi,$cspj]}"
                
                CallSpider

            done

        done
        wait

    else
        echo ""
        
        CallSpider

    fi

#}

#get_cloud
