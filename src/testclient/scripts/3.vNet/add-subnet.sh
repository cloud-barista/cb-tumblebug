#!/bin/bash

function CallTB() {
	echo "- Create subnet in ${MCIRRegionNativeName}"

	CIDRNum=$(($INDEX+1))
	CIDRDiff=$(($CIDRNum*$REGION))
	CIDRDiff=$(($CIDRDiff%254))
	# CIDRDiff=$(($CIDRDiff+1))

    resp=$(
        curl -H "${AUTH}" -sX POST http://$TumblebugServer/tumblebug/ns/$NSID/resources/vNet/${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX}/subnet -H 'Content-Type: application/json' -d @- <<EOF
        {
			"name": "${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX}-02",
			"connectionName": "${CONN_CONFIG[$INDEX,$REGION]}",
			"IPv4_CIDR": "10.${CIDRDiff}.128.0/18"			
		}
EOF
    ); echo ${resp} | jq ''
    echo ""
}

#function create_subnet() {

	echo "####################################################################"
	echo "## 3. subnet: Create and add to vNet"
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

				CallTB

			done

		done
		wait

	else
		echo ""
		
		MCIRRegionNativeName=${CONN_CONFIG[$INDEX,$REGION]}

		CallTB

	fi
	
#}

#create_subnet