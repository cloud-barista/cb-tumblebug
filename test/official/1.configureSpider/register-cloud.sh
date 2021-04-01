#!/bin/bash

#function register_cloud() {
    FILE=../conf.env
    if [ ! -f "$FILE" ]; then
        echo "$FILE does not exist."
        exit
    fi

    FILE=../credentials.conf
    if [ ! -f "$FILE" ]; then
        echo "$FILE does not exist."
        exit
    fi

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
    curl -H "${AUTH}" -sX POST http://$SpiderServer/spider/driver -H 'Content-Type: application/json' -d \
        '{
            "ProviderName" : "'${ProviderName[INDEX]}'",
            "DriverLibFileName" : "'${DriverLibFileName[INDEX]}'",
            "DriverName" : "'${DriverName[INDEX]}'"
        }' | json_pp

    # for Cloud Credential Info
    curl -H "${AUTH}" -sX POST http://$SpiderServer/spider/credential -H 'Content-Type: application/json' -d \
        "{
            \"ProviderName\" : \"${ProviderName[INDEX]}\",
            \"CredentialName\" : \"${CredentialName[INDEX]}\",
            \"KeyValueInfoList\" : [
                {
                    \"Key\" : \"${CredentialKey01[INDEX]:-NULL}\",
                    \"Value\" : \"${CredentialVal01[INDEX]:-NULL}\"
                },
                {
                    \"Key\" : \"${CredentialKey02[INDEX]:-NULL}\",
                    \"Value\" : \"${CredentialVal02[INDEX]:-NULL}\"
                },
                {
                    \"Key\" : \"${CredentialKey03[INDEX]:-NULL}\",
                    \"Value\" : \"${CredentialVal03[INDEX]:-NULL}\"
                },
                {
                    \"Key\" : \"${CredentialKey04[INDEX]:-NULL}\",
                    \"Value\" : \"${CredentialVal04[INDEX]:-NULL}\"
                },
                {
                    \"Key\" : \"${CredentialKey05[INDEX]:-NULL}\",
                    \"Value\" : \"${CredentialVal05[INDEX]:-NULL}\"
                }
            ]
        }" #| json_pp

    # for Cloud Region Info

    if [ "${CSP}" == "azure" ]; then
        # Differenciate Cloud Region Value for Resource Group Name
        curl -H "${AUTH}" -sX POST http://$SpiderServer/spider/region -H 'Content-Type: application/json' -d \
        '{
            "ProviderName" : "'${ProviderName[INDEX]}'",
            "KeyValueInfoList" : [
                {
                    "Key" : "'${RegionKey01[$INDEX,$REGION]:-NULL}'",
                    "Value" : "'${RegionVal01[$INDEX,$REGION]:-NULL}'"
                },
                {
                    "Key" : "'${RegionKey02[$INDEX,$REGION]:-NULL}'",
                    "Value" : "'${RegionVal02[$INDEX,$REGION]:-NULL}'-'${CONN_CONFIG[$INDEX,$REGION]}'-'${POSTFIX}'"
                }
            ],
            "RegionName" : "'${RegionName[$INDEX,$REGION]}'"
        }' | json_pp
    else
        curl -H "${AUTH}" -sX POST http://$SpiderServer/spider/region -H 'Content-Type: application/json' -d \
        '{
            "ProviderName" : "'${ProviderName[INDEX]}'",
            "KeyValueInfoList" : [
                {
                    "Key" : "'${RegionKey01[$INDEX,$REGION]:-NULL}'",
                    "Value" : "'${RegionVal01[$INDEX,$REGION]:-NULL}'"
                },
                {
                    "Key" : "'${RegionKey02[$INDEX,$REGION]:-NULL}'",
                    "Value" : "'${RegionVal02[$INDEX,$REGION]:-NULL}'"
                }
            ],
            "RegionName" : "'${RegionName[$INDEX,$REGION]}'"
        }' | json_pp
    fi


    # for Cloud Connection Config Info
    curl -H "${AUTH}" -sX POST http://$SpiderServer/spider/connectionconfig -H 'Content-Type: application/json' -d \
        '{
            "CredentialName" : "'${CredentialName[INDEX]}'",
            "ConfigName" : "'${CONN_CONFIG[$INDEX,$REGION]}'",
            "ProviderName" : "'${ProviderName[INDEX]}'",
            "DriverName" : "'${DriverName[INDEX]}'",
            "RegionName" : "'${RegionName[$INDEX,$REGION]}'"
        }' | json_pp
#}

#register_cloud