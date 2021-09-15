#!/bin/bash

#function list_cloud() {

    source ../conf.env
    
    echo "####################################################################"
    echo "## 0. List Cloud Connction Config(s)"
    echo "####################################################################"

    # for Cloud Region Info
    echo "[Cloud Region]"
    curl -H "${AUTH}" -sX GET http://$SpiderServer/spider/region |
    jq '.region' |
    jq -r '(["RegionName","ProviderName","Region","Zone"] | (., map(length*"-"))), (.[] | [.RegionName, .ProviderName, .KeyValueInfoList[0].Value, .KeyValueInfoList[1].Value]) | @tsv' |
    column -t
    echo ""
    echo ""


    # for Cloud Credential Info
    echo "[Cloud Credential]"
    curl -H "${AUTH}" -sX GET http://$SpiderServer/spider/credential |
    jq '.credential' |
    jq -r '(["CredentialName","ProviderName"] | (., map(length*"-"))), (.[] | [.CredentialName, .ProviderName]) | @tsv' |
    column -t
    echo ""
    echo ""
    
    
    # for Cloud Driver Info
    echo "[Cloud Driver]"
    curl -H "${AUTH}" -sX GET http://$SpiderServer/spider/driver |
    jq '.driver' |
    jq -r '(["DriverName","ProviderName","DriverLibFileName"] | (., map(length*"-"))), (.[] | [.DriverName, .ProviderName, .DriverLibFileName]) | @tsv' |
    column -t
    echo ""
    echo ""


    # for Cloud Connection Config Info
    echo "[Cloud Connection Config]"
    curl -H "${AUTH}" -sX GET http://$SpiderServer/spider/connectionconfig |
    jq '.connectionconfig' |
    jq -r '(["ConfigName","RegionName","CredentialName","DriverName","ProviderName"] | (., map(length*"-"))), (.[] | [.ConfigName, .RegionName, .CredentialName, .DriverName, .ProviderName]) | @tsv' |
    column -t
    echo ""
#}

#list_cloud
