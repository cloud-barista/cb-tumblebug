#!/bin/bash

function CallSpider() {
    echo "- Delete vNet in ${MCIRRegionName}"

    resp=$(
        curl -H "${AUTH}" -sX DELETE http://$SpiderServer/spider/vpc/${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX}?force=true -H 'Content-Type: application/json' -d '{ "ConnectionName": "'${CONN_CONFIG[$INDEX,$REGION]}'"}' 
    ); echo ${resp} | jq ''
    echo ""
}

#function spider_delete_vNet() {

	echo "####################################################################"
	echo "## 3. vNet: Delete"
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

#spider_delete_vNet
