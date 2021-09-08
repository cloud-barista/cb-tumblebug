#!/bin/bash

function CallSpider() {
    echo "- Create vNet in ${MCIRRegionName}"
    
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
    ); echo ${resp} | jq ''
    echo ""
}

#function create_vNet() {

	echo "####################################################################"
	echo "## 3. vNet: Create"
	echo "####################################################################"

	source ../init.sh

	if [ "${INDEX}" == "0" ]; then
		echo "[Parallel excution for all CSP regions]"

		INDEXX=${NumCSP}
		for ((cspi = 1; cspi <= INDEXX; cspi++)); do
			echo $i
			INDEXY=${NumRegion[$cspi]}
			CSP=${CSPType[$cspi]}
			for ((cspj = 1; cspj <= INDEXY; cspj++)); do
				# INDEX=$(($INDEX+1))

				echo $j
				INDEX=$cspi
				REGION=$cspj
				echo $CSP
				echo $REGION
				echo ${RegionName[$cspi,$cspj]}
				MCIRRegionName=${RegionName[$cspi,$cspj]}

				CallSpider

			done

		done
		wait

	else
		echo ""
		
		MCIRRegionName=${CONN_CONFIG[$INDEX,$REGION]}

		CallSpider

	fi
        
#}

#create_vNet
