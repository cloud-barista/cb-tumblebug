#!/bin/bash

function CallTB() {
	echo "- Register spec in ${ResourceRegionNativeName}"

	resp=$(
        curl -H "${AUTH}" -sX POST http://$TumblebugServer/tumblebug/ns/$NSID/resources/spec -H 'Content-Type: application/json' -d @- <<EOF
		{ 
			"connectionName": "${CONN_CONFIG[$INDEX,$REGION]}", 
			"name": "${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX}",
			"cspSpecName": "${SPEC_NAME[$INDEX,$REGION]}"
		}
EOF
    ); echo ${resp} | jq '.'
    echo ""
}

#function register_spec() {

	echo "####################################################################"
	echo "## 7. spec: Register"
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

#}

#register_spec
