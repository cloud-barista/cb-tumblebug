#!/bin/bash

function CallSpider() {
    echo "- Delete vNet in ${ResourceRegionNativeName}"

    resp=$(
        curl -H "${AUTH}" -sX DELETE http://$SpiderServer/spider/vpc/$NSID-${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX}?force=true -H 'Content-Type: application/json' -d '{ "ConnectionName": "'${CONN_CONFIG[$INDEX,$REGION]}'"}' 
    ); echo ${resp} | jq '.'
    echo ""
}

#function spider_delete_vNet() {

	echo "####################################################################"
	echo "## 3. vNet: Delete"
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
				CallSpider
			done
		done
		wait

	else
		echo ""
		
		ResourceRegionNativeName=${CONN_CONFIG[$INDEX,$REGION]}

		CallSpider

	fi
    
#}

#spider_delete_vNet
