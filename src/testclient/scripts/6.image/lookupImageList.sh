#!/bin/bash

function CallTB() {
	echo "- Lookup images in ${MCIRRegionName}"

	resp=$(
        curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/lookupImages -H 'Content-Type: application/json' -d @- <<EOF
		{ 
			"connectionName": "${CONN_CONFIG[$INDEX,$REGION]}"
		}
EOF
    ); echo ${resp} | jq ''
    echo ""
}

#function lookup_spec_list() {

	echo "####################################################################"
	echo "## 6. image: Lookup Image List"
	echo "####################################################################"

	source ../init.sh

	if [ "${INDEX}" == "0" ]; then
        echo "[Parallel excution for all CSP regions]"
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

#lookup_spec_list
