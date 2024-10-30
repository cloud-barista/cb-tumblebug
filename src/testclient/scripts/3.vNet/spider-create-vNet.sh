#!/bin/bash

function CallSpider() {
    echo "- Create vNet in ${ResourceRegionNativeName}"
    
    resp=$(
            curl -H "${AUTH}" -sX POST http://$SpiderServer/spider/vpc -H 'Content-Type: application/json' -d @- <<EOF
            {
                "ConnectionName": "${CONN_CONFIG[$INDEX,$REGION]}",
                "ReqInfo": {
                    "Name": "${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX}",
                    "IPv4_CIDR": "192.168.0.0/16",
                    "SubnetInfoList": [ {
                        "Name": "${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX}",
                        "IPv4_CIDR": "192.168.1.0/24"

                    } ]

                }
            }
EOF
    ); echo ${resp} | jq '.'
    echo ""
}

#function create_vNet() {

	echo "####################################################################"
	echo "## 3. vNet: Create"
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

				ResourceRegionNativeName=${RegionNativeName[$cspi,$cspj]}

				INDEX=$cspi
				REGION=$cspj
				CallSpider
			done
		done
		wait

	else
		echo ""
		
		ResourceRegionNativeName=${CONN_CONFIG[$INDEX,$REGION]}

		CallSpider

	fi
        
#}

#create_vNet
