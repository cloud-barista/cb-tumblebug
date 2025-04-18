#!/bin/bash

function CallTB() {
	echo "- Get region in ${ResourceRegionNativeName}"

	# for Cloud Region Info
    curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/region/${RegionNativeName[$INDEX,$REGION]} | jq '.'
}

#function get_cloud() {

    echo "####################################################################"
    echo "## 0. Get Cloud Connction Config"
    echo "####################################################################"

    source ../init.sh

	if [ "${INDEX}" == "0" ]; then
		echo "[Parallel execution for all CSP regions]"

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
				echo ${RegionNativeName[$cspi,$cspj]}
				ResourceRegionNativeName=${RegionNativeName[$cspi,$cspj]}

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

#get_cloud
