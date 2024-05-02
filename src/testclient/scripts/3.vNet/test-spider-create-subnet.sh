#!/bin/bash

function CallSpider() {
    echo "- Create subnet in ${MCIRRegionNativeName}"
    
    resp=$(
            curl -H "${AUTH}" -sX POST http://$SpiderServer/spider/vpc/${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX}/subnet -H 'Content-Type: application/json' -d @- <<EOF
            {
                "ConnectionName": "${CONN_CONFIG[$INDEX,$REGION]}",
                "ReqInfo": {
                    "Name": "jhseo-3rd-subnet",
                    "IPv4_CIDR": "192.168.xx.xx/16",
                    "KeyValueList": []

                }
            }
EOF
    ); echo ${resp} | jq ''
    echo ""
}

#function create_subnet() {

	echo "####################################################################"
	echo "## 3. subnet: Create"
	echo "####################################################################"

	source ../init.sh

	if [ "${INDEX}" == "0" ]; then
        echo "[Parallel execution for all CSP regions]"
        INDEXX=${NumCSP}
        for ((cspi = 1; cspi <= INDEXX; cspi++)); do
            INDEXY=${NumRegion[$cspi]}
            CSP=${CSPType[$cspi]}
            echo "[$cspi] $CSP details"
            for ((cspj = 1; cspj <= INDEXY; cspj++)); do
                echo "[$cspi,$cspj] ${RegionNativeName[$cspi,$cspj]}"

				MCIRRegionNativeName=${RegionNativeName[$cspi,$cspj]}

				CallSpider

			done

		done
		wait

	else
		echo ""
		
		MCIRRegionNativeName=${CONN_CONFIG[$INDEX,$REGION]}

		CallSpider

	fi
        
#}

#create_vNet
