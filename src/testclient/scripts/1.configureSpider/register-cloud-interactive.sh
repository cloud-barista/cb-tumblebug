#!/bin/bash

function CallSpiderPostDriver() {
    # for Cloud Driver Info
    # echo "[Cloud Driver] ${DriverName[$INDEX]}"
    resp=$(
        curl -H "${AUTH}" -sX POST http://$SpiderServer/spider/driver -H 'Content-Type: application/json' -d @- <<EOF
        {
             "ProviderName" : "${ProviderName[$INDEX]}",
             "DriverLibFileName" : "${DriverLibFileName[$INDEX]}",
             "DriverName" : "${DriverName[$INDEX]}"
         }
EOF
    )
    # echo ${resp} |
    #     jq -r '(["DriverName","ProviderName","DriverLibFileName"] | (., map(length*"-"))), ([.DriverName, .ProviderName, .DriverLibFileName]) | @tsv' |
    #     column -t
    # echo ""

    # for Cloud Credential Info
    # echo "[Cloud Credential] ${CredentialName[$INDEX]}"
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
                 },
                 {
                     "Key" : "${CredentialKey06[$INDEX]:-NULL}",
                     "Value" : "${CredentialVal06[$INDEX]:-NULL}"
                 },
                 {
                     "Key" : "${CredentialKey07[$INDEX]:-NULL}",
                     "Value" : "${CredentialVal07[$INDEX]:-NULL}"
                 },
                 {
                     "Key" : "${CredentialKey08[$INDEX]:-NULL}",
                     "Value" : "${CredentialVal08[$INDEX]:-NULL}"
                 }
             ]
         }
EOF
    )
    # echo ${resp} | # jq '.message'
    #     jq -r '(["CredentialName","ProviderName"] | (., map(length*"-"))), ([.CredentialName, .ProviderName]) | @tsv' |
    #     column -t
    # echo ""

}
function CallSpiderPostRegion() {
    # for Cloud Region Info
    # Differenciate Cloud Region Value for Resource Group Name
    if [ "${CSP}" == "azure" ]; then
        # echo "[Cloud Region] ${RegionName[$INDEX,$REGION]}"
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
        # echo ${resp} |
        #     jq -r '(["RegionName","ProviderName","Region","Zone"] | (., map(length*"-"))), ([.RegionName, .ProviderName, .KeyValueInfoList[0].Value, .KeyValueInfoList[1].Value]) | @tsv' |
        #     column -t
        # echo ""
    else
        # echo "[Cloud Region] ${RegionName[$INDEX,$REGION]}"
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
        # echo ${resp} |
        #     jq -r '(["RegionName","ProviderName","Region","Zone"] | (., map(length*"-"))), ([.RegionName, .ProviderName, .KeyValueInfoList[0].Value, .KeyValueInfoList[1].Value]) | @tsv' |
        #     column -t
        # echo ""
    fi

    # for Cloud Connection Config Info
    echo "[Cloud Connection Config] ${CONN_CONFIG[$INDEX,$REGION]}"
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
    # echo ${resp} |
    #     jq -r '(["ConfigName","RegionName","CredentialName","DriverName","ProviderName"] | (., map(length*"-"))), ([.ConfigName, .RegionName, .CredentialName, .DriverName, .ProviderName]) | @tsv' |
    #     column -t
    # echo ""
}

echo "####################################################################"
echo "## 1. Register Cloud Inforation"
echo "####################################################################"

SCRIPT_DIR=`dirname ${BASH_SOURCE[0]-$0}`
cd $SCRIPT_DIR

source ../init.sh
echo ""
echo -e "[${BOLD}Configuration${NC} in ${GREEN}${BOLD} $TestSetFile${NC} & ${GREEN}${BOLD} ../conf.env ${NC} files]"
echo ""
echo -e "${BOLD}1) System Endpoints${NC}"
echo -e " - Tumblebug Server : ${GREEN}${BOLD} $TumblebugServer ${NC}"
echo -e " - Spider Server : ${GREEN}${BOLD} $SpiderServer ${NC}"
echo ""

INDEXX=${TotalNumCSP}
echo -e "${BOLD}2) Enabled Clouds and Regions${NC}"

for ((cspi = 1; cspi <= INDEXX; cspi++)); do
	INDEXY=${TotalNumRegion[$cspi]}
	CSP=${CSPType[$cspi]}
	echo -e "${GREEN}${BOLD} - [$cspi] Cloud : $CSP (enabled regions : $INDEXY)${NC}"
	for ((cspj = 1; cspj <= INDEXY; cspj++)); do
		echo -e "${BLUE}${BOLD}   [$cspi,$cspj] Region : ${RegionName[$cspi,$cspj]} (${RegionLocation[$cspi,$cspj]}) ${NC}" 
	done
	echo ""
done

echo -e "${BOLD}"
while true; do
    read -p 'Confirm the above configuration. Do you want to proceed ? (y/n) : ' CHECKPROCEED
    echo -e "${NC}"
    case $CHECKPROCEED in
    [Yy]*)
        break
        ;;
    [Nn]*)
        echo
        echo "Cancel [$0 $@]"
        echo "See you soon. :)"
        echo
        exit 1
        ;;
    *)
        echo "Please answer yes or no."
        ;;
    esac
done

if [ "${INDEX}" == "0" ]; then
    echo "[Parallel execution for all CSP regions]"
    INDEXX=${TotalNumCSP}
    for ((cspi = 1; cspi <= INDEXX; cspi++)); do
        INDEXY=${TotalNumRegion[$cspi]}
        CSP=${CSPType[$cspi]}
        # echo "[$cspi] $CSP details"
        CallSpiderPostDriver
        for ((cspj = 1; cspj <= INDEXY; cspj++)); do
            # echo "[$cspi,$cspj] ${RegionName[$cspi,$cspj]}"

            INDEX=$cspi
            REGION=$cspj
            CallSpiderPostRegion

        done

    done
    wait

else
    echo ""
    CallSpiderPostDriver
    CallSpiderPostRegion

fi

# Print list of all registered cloud info

./list-cloud.sh

echo -e "${NC}"
