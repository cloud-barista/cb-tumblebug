#!/bin/bash

function CallSpider() {
    # for Cloud Region Info
    echo "[Cloud Region] ${RegionNativeName[$INDEX,$REGION]}"
    curl -H "${AUTH}" -sX GET http://$SpiderServer/spider/region/${RegionNativeName[$INDEX,$REGION]} |
    jq -r '(["RegionNativeName","ProviderName","Region","Zone"] | (., map(length*"-"))), ([.RegionNativeName, .ProviderName, .KeyValueInfoList[0].Value, .KeyValueInfoList[1].Value]) | @tsv' |
    column -t
    echo ""
    echo ""


    # for Cloud Credential Info
    echo "[Cloud Credential] ${CredentialName[$INDEX]}"
    curl -H "${AUTH}" -sX GET http://$SpiderServer/spider/credential/${CredentialName[$INDEX]} |
    jq -r '(["CredentialName","ProviderName"] | (., map(length*"-"))), ([.CredentialName, .ProviderName]) | @tsv' |
    column -t
    echo ""
    echo ""

    
    # for Cloud Driver Info
    echo "[Cloud Driver] ${DriverName[$INDEX]}"
    curl -H "${AUTH}" -sX GET http://$SpiderServer/spider/driver/${DriverName[$INDEX]} |
    jq -r '(["DriverName","ProviderName","DriverLibFileName"] | (., map(length*"-"))), ([.DriverName, .ProviderName, .DriverLibFileName]) | @tsv' |
    column -t
    echo ""
    echo ""


    # for Cloud Connection Config Info
    echo "[Cloud Connection Config] ${CONN_CONFIG[$INDEX,$REGION]}"
    curl -H "${AUTH}" -sX GET http://$SpiderServer/spider/connectionconfig/${CONN_CONFIG[$INDEX,$REGION]} |
    jq -r '(["ConfigName","RegionNativeName","CredentialName","DriverName","ProviderName"] | (., map(length*"-"))), ([.ConfigName, .RegionNativeName, .CredentialName, .DriverName, .ProviderName]) | @tsv' |
    column -t
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
                echo "[$cspi,$cspj] ${RegionNativeName[$cspi,$cspj]}"
                
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
