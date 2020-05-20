#!/bin/bash
source ../conf.env
source ../credentials.conf

echo "####################################################################"
echo "## 0. Create Cloud Connction Config"
echo "####################################################################"

CSP=${1}
POSTFIX=${2:-developer}
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

RESTSERVER=localhost

 # for Cloud Driver Info
curl -sX POST http://$RESTSERVER:1024/spider/driver -H 'Content-Type: application/json' -d \
	'{
        "ProviderName" : "'${ProviderName[INDEX]}'",
        "DriverLibFileName" : "'${DriverLibFileName[INDEX]}'",
        "DriverName" : "'${DriverName[INDEX]}'"
	}' | json_pp

 # for Cloud Credential Info
curl -sX POST http://$RESTSERVER:1024/spider/credential -H 'Content-Type: application/json' -d \
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
    }" | json_pp

 # for Cloud Region Info

if [ "${CSP}" == "azure" ]; then
    # Differenciate Cloud Region Value for Resource Group Name
	curl -sX POST http://$RESTSERVER:1024/spider/region -H 'Content-Type: application/json' -d \
    '{
        "ProviderName" : "'${ProviderName[INDEX]}'",
        "KeyValueInfoList" : [
            {
                "Key" : "'${RegionKey01[INDEX]:-NULL}'",
                "Value" : "'${RegionVal01[INDEX]:-NULL}'"
            },
            {
                "Key" : "'${RegionKey02[INDEX]:-NULL}'",
                "Value" : "'${RegionVal02[INDEX]:-NULL}'-'$CSP'-'$POSTFIX'"
            }
        ],
        "RegionName" : "'${RegionName[INDEX]}'"
    }' | json_pp
else
curl -sX POST http://$RESTSERVER:1024/spider/region -H 'Content-Type: application/json' -d \
    '{
        "ProviderName" : "'${ProviderName[INDEX]}'",
        "KeyValueInfoList" : [
            {
                "Key" : "'${RegionKey01[INDEX]:-NULL}'",
                "Value" : "'${RegionVal01[INDEX]:-NULL}'"
            },
            {
                "Key" : "'${RegionKey02[INDEX]:-NULL}'",
                "Value" : "'${RegionVal02[INDEX]:-NULL}'"
            }
        ],
        "RegionName" : "'${RegionName[INDEX]}'"
    }' | json_pp
fi


 # for Cloud Connection Config Info
curl -sX POST http://$RESTSERVER:1024/spider/connectionconfig -H 'Content-Type: application/json' -d \
    '{
        "CredentialName" : "'${CredentialName[INDEX]}'",
        "ConfigName" : "'${CONN_CONFIG[INDEX]}'",
        "ProviderName" : "'${ProviderName[INDEX]}'",
        "DriverName" : "'${DriverName[INDEX]}'",
        "RegionName" : "'${RegionName[INDEX]}'"
    }' | json_pp
