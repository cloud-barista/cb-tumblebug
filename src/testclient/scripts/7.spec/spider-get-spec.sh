#!/bin/bash

function CallSpider() {
	echo "- Get spec in ${MCIRRegionNativeName}"
	
	curl -H "${AUTH}" -sX GET http://$SpiderServer/spider/vmspec/${SPEC_NAME[$INDEX,$REGION]} -H 'Content-Type: application/json' -d '{ "ConnectionName": "'${CONN_CONFIG[$INDEX,$REGION]}'" }' | jq ''
}

#function spider_get_spec() {

	echo "####################################################################"
	echo "## 7. spec: Get"
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

				CallSpider

			done

		done
		wait

	else
		echo ""
		
		MCIRRegionNativeName=${CONN_CONFIG[$INDEX,$REGION]}

		CallSpider

	fi
	
#}

#spider_get_spec