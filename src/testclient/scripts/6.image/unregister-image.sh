#!/bin/bash

function CallTB() {
	echo "- Unregister image in ${MCIRRegionName}"

	curl -H "${AUTH}" -sX DELETE http://$TumblebugServer/tumblebug/ns/$NSID/resources/image/${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX} | jq ''

	if [ -n "${CONTAINER_IMAGE_NAME[$INDEX,$REGION]}" ]; then
		curl -H "${AUTH}" -sX DELETE http://$TumblebugServer/tumblebug/ns/$NSID/resources/image/k8s-${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX} | jq ''
	fi
}

#function unregister_image() {

	echo "####################################################################"
	echo "## 6. image: Unregister"
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

#unregister_image
