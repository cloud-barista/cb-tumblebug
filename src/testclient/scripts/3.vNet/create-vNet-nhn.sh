#!/bin/bash

function CallTB() {
	echo "- Create vNet in ${MCIRRegionName}"

    resp=$(
        curl -H "${AUTH}" -sX POST http://$TumblebugServer/tumblebug/ns/$NSID/resources/vNet?option=register -H 'Content-Type: application/json' -d @- <<EOF
        {
			"name": "${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX}",
			"connectionName": "${CONN_CONFIG[$INDEX,$REGION]}",
			"cspVNetId": "7e3af4cc-407b-47f6-beda-7c161ebe56f0"
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

#create_vNet