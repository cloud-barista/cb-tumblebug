#!/bin/bash

function CallTB() {
	echo "- Get NLB in ${MCIRRegionName}"

	curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NSID/nlb/${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX} | jq ''
    echo ""
}

#function get_NLB() {

	echo "####################################################################"
	echo "## 10. NLB: Get"
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

#get_NLB
