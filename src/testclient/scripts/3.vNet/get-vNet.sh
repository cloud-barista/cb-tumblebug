#!/bin/bash

function CallTB() {
	echo "- Get vNet in ${ResourceRegionNativeName}"

	curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NSID/resources/vNet/${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX} | jq '.'
    echo ""
}

#function get_vNet() {

	echo "####################################################################"
	echo "## 3. vNet: Get"
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

#get_vNet
