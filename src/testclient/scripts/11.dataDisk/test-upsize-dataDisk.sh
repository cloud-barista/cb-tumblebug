#!/bin/bash

function CallTB() {
	echo "- Upsize dataDisk in ${MCIRRegionName}"

	curl -H "${AUTH}" -sX PUT http://$TumblebugServer/tumblebug/ns/$NSID/resources/dataDisk/${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX} -H 'Content-Type: application/json' -d \
		'{
			"diskSize": "81",
			"description": "UpsizeDataDisk() test"
		}' | jq ''
}

#function update_dataDisk() {

	echo "####################################################################"
	echo "## 11. dataDisk: Upsize"
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

#update_dataDisk