#!/bin/bash

function CallSpider() {
    echo "- Get sshKey in ${ResourceRegionNativeName}"

    resp=$(
        curl -H "${AUTH}" -sX GET http://$SpiderServer/spider/keypair/${NSID}-${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX}?force=true -H 'Content-Type: application/json' -d @- <<EOF
        { 
			"ConnectionName": "${CONN_CONFIG[$INDEX,$REGION]}"
		}
EOF
    ); echo ${resp} | jq ''
    echo ""
}

#function spider_get_sshKey() {

    echo "####################################################################"
	echo "## 5. sshKey: Get"
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

#spider_get_sshKey