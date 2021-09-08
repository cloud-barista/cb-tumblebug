#!/bin/bash

function CallSpider() {
# for Cloud Driver Info
resp=$(
    curl -H "${AUTH}" -sX POST http://$SpiderServer/spider/driver -H 'Content-Type: application/json' -d @- <<EOF
        {
             "ProviderName" : "${ProviderName[$INDEX]}",
             "DriverLibFileName" : "${DriverLibFileName[$INDEX]}",
             "DriverName" : "${DriverName[$INDEX]}"
         }
EOF
)
echo ${resp} | jq ''
echo ""

# for Cloud Credential Info
resp=$(
    curl -H "${AUTH}" -sX POST http://$SpiderServer/spider/credential -H 'Content-Type: application/json' -d @- <<EOF
        {
             "ProviderName" : "${ProviderName[$INDEX]}",
             "CredentialName" : "${CredentialName[$INDEX]}",
             "KeyValueInfoList" : [
                 {
                     "Key" : "${CredentialKey01[$INDEX]:-NULL}",
                     "Value" : "${CredentialVal01[$INDEX]:-NULL}"
                 },
                 {
                     "Key" : "${CredentialKey02[$INDEX]:-NULL}",
                     "Value" : "${CredentialVal02[$INDEX]:-NULL}"
                 },
                 {
                     "Key" : "${CredentialKey03[$INDEX]:-NULL}",
                     "Value" : "${CredentialVal03[$INDEX]:-NULL}"
                 },
                 {
                     "Key" : "${CredentialKey04[$INDEX]:-NULL}",
                     "Value" : "${CredentialVal04[$INDEX]:-NULL}"
                 },
                 {
                     "Key" : "${CredentialKey05[$INDEX]:-NULL}",
                     "Value" : "${CredentialVal05[$INDEX]:-NULL}"
                 }
             ]
         }
EOF
)
echo ${resp} | jq '.message'
echo ""

# for Cloud Region Info
# Differenciate Cloud Region Value for Resource Group Name
if [ "${CSP}" == "azure" ]; then
    resp=$(
        curl -H "${AUTH}" -sX POST http://$SpiderServer/spider/region -H 'Content-Type: application/json' -d @- <<EOF
            {
            "ProviderName" : "${ProviderName[$INDEX]}",
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
    )
    echo ${resp} | jq ''
    echo ""
else
    resp=$(
        curl -H "${AUTH}" -sX POST http://$SpiderServer/spider/region -H 'Content-Type: application/json' -d @- <<EOF
            {
            "ProviderName" : "${ProviderName[$INDEX]}",
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
    )
    echo ${resp} | jq ''
    echo ""
fi

# for Cloud Connection Config Info
resp=$(
    curl -H "${AUTH}" -sX POST http://$SpiderServer/spider/connectionconfig -H 'Content-Type: application/json' -d @- <<EOF
        {
            "ConfigName" : "${CONN_CONFIG[$INDEX,$REGION]}",
            "CredentialName" : "${CredentialName[$INDEX]}",
            "ProviderName" : "${ProviderName[$INDEX]}",
            "DriverName" : "${DriverName[$INDEX]}",
            "RegionName" : "${RegionName[$INDEX,$REGION]}"
        }
EOF
)
echo ${resp} | jq ''
echo ""
}

#function register_cloud() {

echo "####################################################################"
echo "## 1. Create Cloud Connction Config"
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

#register_cloud
