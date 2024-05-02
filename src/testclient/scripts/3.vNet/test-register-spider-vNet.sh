#!/bin/bash

function CallTB() {
	echo "- Register vNet in ${MCIRRegionNativeName}"
	echo "{CONN_CONFIG[$INDEX,$REGION]}: ${CONN_CONFIG[$INDEX,$REGION]}"
	
    resp=$(
        curl -H "${AUTH}" -sX POST http://$TumblebugServer/tumblebug/ns/$NSID/resources/vNet?option=register -H 'Content-Type: application/json' -d @- <<EOF
        {
			"name": "${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX}",
			"connectionName": "${CONN_CONFIG[$INDEX,$REGION]}"
		}
EOF
    ); echo ${resp} | jq ''
    echo ""
}

#function register_vNet() {

	echo "####################################################################"
	echo "## 3. vNet: Register"
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

#register_vNet