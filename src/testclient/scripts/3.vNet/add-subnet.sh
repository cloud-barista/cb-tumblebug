#!/bin/bash

function CallTB() {
	echo "- Create subnet in ${MCIRRegionName}"

	CIDRNum=$(($INDEX+1))
	CIDRDiff=$(($CIDRNum*$REGION))
	CIDRDiff=$(($CIDRDiff%254))
	# CIDRDiff=$(($CIDRDiff+1))

    resp=$(
        curl -H "${AUTH}" -sX POST http://$TumblebugServer/tumblebug/ns/$NSID/resources/vNet/${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX}/subnet -H 'Content-Type: application/json' -d @- <<EOF
        {
			"name": "${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX}-${CIDRDiff}",
			"connectionName": "${CONN_CONFIG[$INDEX,$REGION]}",
			"IPv4_CIDR": "192.168.${CIDRDiff}.64/28"
			
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

#create_subnet