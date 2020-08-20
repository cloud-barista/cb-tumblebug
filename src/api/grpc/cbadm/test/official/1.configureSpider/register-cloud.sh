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
    if [ "${CSP}" == "aws" ]; then
        echo "[Test for AWS]"
        INDEX=1
    elif [ "${CSP}" == "azure" ]; then
        echo "[Test for Azure]"
        INDEX=2
    elif [ "${CSP}" == "gcp" ]; then
        echo "[Test for GCP]"
        INDEX=3
    elif [ "${CSP}" == "alibaba" ]; then
        echo "[Test for Alibaba]"
        INDEX=4
    else
        echo "[No acceptable argument was provided (aws, azure, gcp, alibaba, ...). Default: Test for AWS]"
        CSP="aws"
        INDEX=1
    fi

    # for Cloud Driver Info
    $CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/cbadm driver create --config $CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/grpc_conf.yaml -i json -o json -d \
        '{
            "ProviderName" : "'${ProviderName[INDEX]}'",
            "DriverLibFileName" : "'${DriverLibFileName[INDEX]}'",
            "DriverName" : "'${DriverName[INDEX]}'"
        }'

    # for Cloud Credential Info
    $CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/cbadm credential create --config $CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/grpc_conf.yaml -i json -o json -d \
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
                }
            ]
        }" 

    # for Cloud Region Info

    if [ "${CSP}" == "azure" ]; then
        # Differenciate Cloud Region Value for Resource Group Name
        $CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/cbadm region create --config $CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/grpc_conf.yaml -i json -o json -d \
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
        }' 
    else
    $CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/cbadm region create --config $CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/grpc_conf.yaml -i json -o json -d \
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
        }'
    fi


    # for Cloud Connection Config Info
    $CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/cbadm connect-infos create --config $CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/grpc_conf.yaml -i json -o json -d \
        '{
            "CredentialName" : "'${CredentialName[INDEX]}'",
            "ConfigName" : "'${CONN_CONFIG[$INDEX,$REGION]}'",
            "ProviderName" : "'${ProviderName[INDEX]}'",
            "DriverName" : "'${DriverName[INDEX]}'",
            "RegionName" : "'${RegionName[$INDEX,$REGION]}'"
        }'
#}

#register_cloud