#!/bin/bash

function CallSpider() {
	# for Cloud Connection Config Info
	curl -H "${AUTH}" -sX DELETE http://$SpiderServer/spider/connectionconfig/${CONN_CONFIG[$INDEX,$REGION]} | jq ''
    echo ""


	# for Cloud Region Info
	curl -H "${AUTH}" -sX DELETE http://$SpiderServer/spider/region/${RegionName[$INDEX,$REGION]} | jq ''
    echo ""


	# for Cloud Credential Info
	curl -H "${AUTH}" -sX DELETE http://$SpiderServer/spider/credential/${CredentialName[$INDEX]} | jq ''
    echo ""


	# for Cloud Driver Info
	curl -H "${AUTH}" -sX DELETE http://$SpiderServer/spider/driver/${DriverName[$INDEX]} | jq ''
    echo ""
}

#function unregister_cloud() {
	
	echo "####################################################################"
	echo "## 1. Remove All Cloud Connction Config(s)"
	echo "####################################################################"

	source ../init.sh

	if [ "${OPTION}" == "leave" ]; then
		echo "[Leave Cloud Credential and Cloud Driver for other Regions]"
		exit
	fi
	
	if [ "${INDEX}" == "0" ]; then
		echo "[Parallel excution for all CSP regions]"

		INDEXX=${NumCSP}
		for ((cspi = 1; cspi <= INDEXX; cspi++)); do
			echo $i
			INDEXY=${NumRegion[$cspi]}
			CSP=${CSPType[$cspi]}
			for ((cspj = 1; cspj <= INDEXY; cspj++)); do
				# INDEX=$(($INDEX+1))

				echo $j
				INDEX=$cspi
				REGION=$cspj
				echo $CSP
				echo $REGION
				echo ${RegionName[$cspi,$cspj]}
				
				CallSpider

			done

		done
		wait

	else
		echo ""
		
		CallSpider

	fi

#}

#unregister_cloud
