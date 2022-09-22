#!/bin/bash

function CallTB() {
	echo "- Remove VM from NLB in ${MCIRRegionName}"

    resp=$(
        curl -H "${AUTH}" -sX DELETE http://$TumblebugServer/tumblebug/ns/$NSID/nlb/${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX}/vm -H 'Content-Type: application/json' -d @- <<EOF
        {
			"targetGroup": {
				"MCIS" : "${MCISID}",
				"VMs" : [
					"${CONN_CONFIG[$INDEX,$REGION]}-0"
					]
			}
		}
EOF
    ); echo ${resp} | jq ''
    echo ""
	# echo ["${CONN_CONFIG[$INDEX,$REGION]}-0"] # for debug
}

#function create_nlb() {

	echo "####################################################################"
	echo "## 10. NLB: Remove VM"
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