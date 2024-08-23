#!/bin/bash

function CallTB() {
	echo "- Create securityGroup in ${ResourceRegionNativeName}"

	resp=$(
        curl -H "${AUTH}" -sX POST http://$TumblebugServer/tumblebug/ns/$NSID/resources/securityGroup?option=register -H 'Content-Type: application/json' -d @- <<EOF
        {
			"connectionName": "${CONN_CONFIG[$INDEX,$REGION]}",
			"name": "${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX}",			
			"vNetId": "${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX}",
			"cspSecurityGroupId": "1333707"
		}
EOF
    ); echo ${resp} | jq ''
    echo ""
}

#function create_securityGroup() {

	echo "####################################################################"
	echo "## 4. SecurityGroup: Create"
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
				CallTB
			done
		done
		wait
	else
		echo ""
		
		ResourceRegionNativeName=${CONN_CONFIG[$INDEX,$REGION]}

		CallTB
	fi
	fi
    
#}

#create_securityGroup
