#!/bin/bash

function CallTB() {
	echo "- Create nlb in ${MCIRRegionName}"

    resp=$(
        curl -H "${AUTH}" -sX POST http://$TumblebugServer/tumblebug/ns/$NSID/nlb -H 'Content-Type: application/json' -d @- <<EOF
        {
			"name": "${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX}",
			"connectionName": "${CONN_CONFIG[$INDEX,$REGION]}",
			"vNetId": "${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX}",
			"type": "PUBLIC",
			"scope": "REGION",
			"listener": {
				"Protocol": "TCP",
				"Port": "22",
				"DNSName": ""
			},
			"targetGroup": {
				"Protocol" : "TCP",
				"Port" : "22", 
				"VMs" : [
					"${CONN_CONFIG[$INDEX,$REGION]}-0",
					"${CONN_CONFIG[$INDEX,$REGION]}-1",
					"${CONN_CONFIG[$INDEX,$REGION]}-2"
					],
				"MCIS" : "${MCISID}"
			},
			"HealthChecker": {
				"Protocol" : "TCP", 
				"Port" : "22", 
				"Interval" : "10", 
				"Timeout" : "-1", 
				"Threshold" : "3" 
			}
		}
EOF
    ); echo ${resp} | jq ''
    echo ""
	# echo ["${CONN_CONFIG[$INDEX,$REGION]}-0"] # for debug
}

#function create_nlb() {

	echo "####################################################################"
	echo "## 10. NLB: Create"
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
                echo "[$cspi,$cspj] ${RegionName[$cspi,$cspj]}"
				
				MCIRRegionName=${RegionName[$cspi,$cspj]}

				CallTB

			done

		done
		wait

	else
		echo ""
		
		MCIRRegionName=${CONN_CONFIG[$INDEX,$REGION]}

		CallTB

	fi
	
#}

#create_nlb