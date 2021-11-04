#!/bin/bash

#function register_cloud() {


    FILE=../../../../conf/credentials.conf
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
    source ../../../../conf/credentials.conf
    
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
        $CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/cbadm driver create --config $CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/grpc_conf.yaml -i json -o json -d \
        "{
            \"ProviderName\" : \"${ProviderName[$INDEX]}\",
            \"DriverLibFileName\" : \"${DriverLibFileName[$INDEX]}\",
            \"DriverName\" : \"${DriverName[$INDEX]}\"
        }"        
    ); echo ${resp} | jq ''
    echo ""

    # for Cloud Credential Info
    resp=$(
        $CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/cbadm credential create --config $CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/grpc_conf.yaml -i json -o json -d \
        "{
             \"ProviderName\" : \"${ProviderName[$INDEX]}\",
             \"CredentialName\" : \"${CredentialName[$INDEX]}\",
             \"KeyValueInfoList\" : [
                 {
                    \"Key\" : \"${CredentialKey01[$INDEX]:-NULL}\",
                    \"Value\" : \"${CredentialVal01[$INDEX]:-NULL}\"
                 },
                 {
                    \"Key\" : \"${CredentialKey02[$INDEX]:-NULL}\",
                    \"Value\" : \"${CredentialVal02[$INDEX]:-NULL}\"
                 },
                 {
                     \"Key\" : \"${CredentialKey03[$INDEX]:-NULL}\",
                     \"Value\" : \"${CredentialVal03[$INDEX]:-NULL}\"
                 },
                 {
                     \"Key\" : \"${CredentialKey04[$INDEX]:-NULL}\",
                     \"Value\" : \"${CredentialVal04[$INDEX]:-NULL}\"
                 },
                 {
                     \"Key\" : \"${CredentialKey05[$INDEX]:-NULL}\",
                     \"Value\" : \"${CredentialVal05[$INDEX]:-NULL}\"
                 }
             ]
         }"       
    ); echo ${resp} | jq '.message'
    echo ""

    # for Cloud Region Info
    # Differenciate Cloud Region Value for Resource Group Name
    if [ "${CSP}" == "azure" ]; then
        resp=$(
            $CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/cbadm region create --config $CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/grpc_conf.yaml -i json -o json -d \
            "{
                \"ProviderName\" : \"${ProviderName[$INDEX]}\",
                \"KeyValueInfoList\" : [
                    {
                        \"Key\" : \"${RegionKey01[$INDEX,$REGION]:-NULL}\",
                        \"Value\" : \"${RegionVal01[$INDEX,$REGION]:-NULL}\"
                    },
                    {
                        \"Key\" : \"${RegionKey02[$INDEX,$REGION]:-NULL}\",
                        \"Value\" : \"${RegionVal02[$INDEX,$REGION]:-NULL}-${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX}\"
                    }
                ],
                \"RegionName\" : \"${RegionName[$INDEX,$REGION]}\"
            }"
        ); echo ${resp} | jq ''
        echo ""
    else
        resp=$(
            $CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/cbadm region create --config $CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/grpc_conf.yaml -i json -o json -d \
            "{
                \"ProviderName\" : \"${ProviderName[$INDEX]}\",
                \"KeyValueInfoList\" : [
                    {
                        \"Key\" : \"${RegionKey01[$INDEX,$REGION]:-NULL}\",
                        \"Value\" : \"${RegionVal01[$INDEX,$REGION]:-NULL}\"
                    },
                    {
                        \"Key\" : \"${RegionKey02[$INDEX,$REGION]:-NULL}\",
                        \"Value\" : \"${RegionVal02[$INDEX,$REGION]:-NULL}\"
                    }
                ],
                \"RegionName\" : \"${RegionName[$INDEX,$REGION]}\"
            }"           
        ); echo ${resp} | jq ''
        echo ""
    fi


    # for Cloud Connection Config Info
    resp=$(
        $CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/cbadm connect-info create --config $CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/grpc_conf.yaml -i json -o json -d \
        "{
            \"CredentialName\" : \"${CredentialName[$INDEX]}\",
            \"ConfigName\" : \"${CONN_CONFIG[$INDEX,$REGION]}\",
            \"ProviderName\" : \"${ProviderName[$INDEX]}\",
            \"DriverName\" : \"${DriverName[$INDEX]}\",
            \"RegionName\" : \"${RegionName[$INDEX,$REGION]}\"
        }"        
    ); echo ${resp} | jq ''
    echo ""
#}

#register_cloud