#!/bin/bash

#function register_cloud() {


    FILE=../credentials.conf
    if [ ! -f "$FILE" ]; then
        echo "$FILE does not exist."
        exit
    fi

    TestSetFile=${4:-../testSet.env}
    
    FILE=$TestSetFile
    if [ ! -f "$FILE" ]; then
        echo "$FILE does not exist."
        exit
    fi
	source $TestSetFile
    source ../conf.env
    source ../credentials.conf
    AUTH="Authorization: Basic $(echo -n $ApiUsername:$ApiPassword | base64)"

    echo "####################################################################"
    echo "## 1. Create Cloud Connction Config"
    echo "####################################################################"

    CSP=${1}
    REGION=${2:-1}
    POSTFIX=${3:-developer}
    
	source ../common-functions.sh
	getCloudIndex $CSP

    RESTSERVER=localhost

    # for Cloud Driver Info
    resp=$(
        curl -H "${AUTH}" -sX POST http://$SpiderServer/spider/driver -H 'Content-Type: application/json' -d @- <<EOF
        {
             "ProviderName" : "${ProviderName[INDEX]}",
             "DriverLibFileName" : "${DriverLibFileName[INDEX]}",
             "DriverName" : "${DriverName[INDEX]}"
         }
EOF
    ); echo ${resp} | jq ''
    echo ""

    # for Cloud Credential Info
    resp=$(
        curl -H "${AUTH}" -sX POST http://$SpiderServer/spider/credential -H 'Content-Type: application/json' -d @- <<EOF
        {
             "ProviderName" : "${ProviderName[INDEX]}",
             "CredentialName" : "${CredentialName[INDEX]}",
             "KeyValueInfoList" : [
                 {
                     "Key" : "${CredentialKey01[INDEX]:-NULL}",
                     "Value" : "${CredentialVal01[INDEX]:-NULL}"
                 },
                 {
                     "Key" : "${CredentialKey02[INDEX]:-NULL}",
                     "Value" : "${CredentialVal02[INDEX]:-NULL}"
                 },
                 {
                     "Key" : "${CredentialKey03[INDEX]:-NULL}",
                     "Value" : "${CredentialVal03[INDEX]:-NULL}"
                 },
                 {
                     "Key" : "${CredentialKey04[INDEX]:-NULL}",
                     "Value" : "${CredentialVal04[INDEX]:-NULL}"
                 },
                 {
                     "Key" : "${CredentialKey05[INDEX]:-NULL}",
                     "Value" : "${CredentialVal05[INDEX]:-NULL}"
                 }
             ]
         }
EOF
    ); echo ${resp} | jq '.message'
    echo ""

    # for Cloud Region Info
    # Differenciate Cloud Region Value for Resource Group Name
    if [ "${CSP}" == "azure" ]; then
        resp=$(
            curl -H "${AUTH}" -sX POST http://$SpiderServer/spider/region -H 'Content-Type: application/json' -d @- <<EOF
            {
            "ProviderName" : "${ProviderName[INDEX]}",
            "KeyValueInfoList" : [
                {
                    "Key" : "${RegionKey01[$INDEX,$REGION]:-NULL}",
                    "Value" : "${RegionVal01[$INDEX,$REGION]:-NULL}"
                },
                {
                    "Key" : "${RegionKey02[$INDEX,$REGION]:-NULL}",
                    "Value" : "${RegionVal02[$INDEX,$REGION]:-NULL}-${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX}"
                }
            ],
            "RegionName" : "${RegionName[$INDEX,$REGION]}"
        }
EOF
        ); echo ${resp} | jq ''
        echo ""
    else
        resp=$(
            curl -H "${AUTH}" -sX POST http://$SpiderServer/spider/region -H 'Content-Type: application/json' -d @- <<EOF
            {
            "ProviderName" : "${ProviderName[INDEX]}",
            "KeyValueInfoList" : [
                {
                    "Key" : "${RegionKey01[$INDEX,$REGION]:-NULL}",
                    "Value" : "${RegionVal01[$INDEX,$REGION]:-NULL}"
                },
                {
                    "Key" : "${RegionKey02[$INDEX,$REGION]:-NULL}",
                    "Value" : "${RegionVal02[$INDEX,$REGION]:-NULL}"
                }
            ],
            "RegionName" : "${RegionName[$INDEX,$REGION]}"
        }
EOF
        ); echo ${resp} | jq ''
        echo ""
    fi


    # for Cloud Connection Config Info
    resp=$(
        curl -H "${AUTH}" -sX POST http://$SpiderServer/spider/connectionconfig -H 'Content-Type: application/json' -d @- <<EOF
        {
            "CredentialName" : "${CredentialName[INDEX]}",
            "ConfigName" : "${CONN_CONFIG[$INDEX,$REGION]}",
            "ProviderName" : "${ProviderName[INDEX]}",
            "DriverName" : "${DriverName[INDEX]}",
            "RegionName" : "${RegionName[$INDEX,$REGION]}"
        }
EOF
    ); echo ${resp} | jq ''
    echo ""
#}

#register_cloud