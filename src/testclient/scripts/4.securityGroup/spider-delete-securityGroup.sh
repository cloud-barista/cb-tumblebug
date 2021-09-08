#!/bin/bash

function CallSpider() {
	echo "- Delete securityGroup in ${MCIRRegionName}"

	curl -H "${AUTH}" -sX DELETE http://$SpiderServer/spider/securitygroup/${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX}?force=true -H 'Content-Type: application/json' -d '{ "ConnectionName": "'${CONN_CONFIG[$INDEX,$REGION]}'"}' | jq ''
}

#function spider_get_securityGroup() {

	echo "####################################################################"
	echo "## 4. securityGroup: Delete"
	echo "####################################################################"

	source ../init.sh

	if [ "${INDEX}" == "0" ]; then
        echo "[Parallel excution for all CSP regions]"
        INDEXX=${NumCSP}
        for ((cspi = 1; cspi <= INDEXX; cspi++)); do
            INDEXY=${NumRegion[$cspi]}
            CSP=${CSPType[$cspi]}
            echo "[$cspi] $CSP details"
            for ((cspj = 1; cspj <= INDEXY; cspj++)); do
                echo "[$cspi,$cspj] ${RegionName[$cspi,$cspj]}"

				MCIRRegionName=${RegionName[$cspi,$cspj]}

				CallSpider

			done

		done
		wait

	else
		echo ""
		
		MCIRRegionName=${CONN_CONFIG[$INDEX,$REGION]}

		CallSpider

	fi
	
#}

#spider_get_securityGroup
