#!/bin/bash

function CallTB() {
	echo "- Delete dataDisk in ${MCIRRegionName}"

	curl -H "${AUTH}" -sX DELETE http://$TumblebugServer/tumblebug/ns/$NSID/resources/dataDisk/${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX} | jq ''
}

#function delete_dataDisk() {

	echo "####################################################################"
	echo "## 11. dataDisk: Delete"
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

#delete_dataDisk