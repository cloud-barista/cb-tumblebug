#!/bin/bash

function CallTB() {
	echo "- Lookup spec in ${ResourceRegionNativeName}"

	resp=$(
        curl -H "${AUTH}" -sX POST http://$TumblebugServer/tumblebug/lookupSpec -H 'Content-Type: application/json' -d @- <<EOF
		{ 
			"connectionName": "${CONN_CONFIG[$INDEX,$REGION]}",
			"cspResourceId": "${SPEC_NAME[$INDEX,$REGION]}"
		}
EOF
    ); echo ${resp} | jq ''
    echo ""
}

#function lookup_spec() {

	echo "####################################################################"
	echo "## 7. spec: Lookup Spec"
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

				INDEX=$cspi
				REGION=$cspj
				ResourceRegionNativeName=${RegionNativeName[$cspi,$cspj]}

				CallTB
			done
		done
		wait

	else
		echo ""
		
		ResourceRegionNativeName=${CONN_CONFIG[$INDEX,$REGION]}

		CallTB

	fi
	
#}

#lookup_spec
