#!/bin/bash

source ../conf.env

echo "####################################################################"
echo "## 0. Create Cloud Connction Config"
echo "####################################################################"

INDEX=${1}

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
    '{
        "ProviderName" : "'${ProviderName[INDEX]}'",
        "CredentialName" : "'${CredentialName[INDEX]}'",
        "KeyValueInfoList" : [
            {
                "Key" : "'${CredentialKey01[INDEX]:-NULL}'",
                "Value" : "'${CredentialVal01[INDEX]:-NULL}'"
            },
            {
                "Key" : "'${CredentialKey02[INDEX]:-NULL}'",
                "Value" : "'${CredentialVal02[INDEX]:-NULL}'"
            },
            {
                "Key" : "'${CredentialKey03[INDEX]:-NULL}'",
                "Value" : "'${CredentialVal03[INDEX]:-NULL}'"
            },
            {
                "Key" : "'${CredentialKey04[INDEX]:-NULL}'",
                "Value" : "'${CredentialVal04[INDEX]:-NULL}'"
            }
        ]
    }' | json_pp

 # for Cloud Region Info
curl -sX POST http://$RESTSERVER:1024/spider/region -H 'Content-Type: application/json' -d \
    '{
        "ProviderName" : "'${ProviderName[INDEX]}'",
        "KeyValueInfoList" : [
            {
                "Key" : "'${RegionKey01[INDEX]:-NULL}'",
                "Value" : "'${RegionVal01[INDEX]:-NULL}'"
            }
        ],
        "RegionName" : "'${RegionName[INDEX]}'"
    }' | json_pp


 # for Cloud Connection Config Info
curl -sX POST http://$RESTSERVER:1024/spider/connectionconfig -H 'Content-Type: application/json' -d \
    '{
        "CredentialName" : "'${CredentialName[INDEX]}'",
        "ConfigName" : "'${CONN_CONFIG[INDEX]}'",
        "ProviderName" : "'${ProviderName[INDEX]}'",
        "DriverName" : "'${DriverName[INDEX]}'",
        "RegionName" : "'${RegionName[INDEX]}'"
    }' | json_pp
