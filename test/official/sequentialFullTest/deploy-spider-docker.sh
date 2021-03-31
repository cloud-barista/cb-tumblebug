#!/bin/bash

#function deploy_nginx_to_mcis() {
    FILE=../conf.env
    if [ ! -f "$FILE" ]; then
        echo "$FILE does not exist."
        exit
    fi

	source ../conf.env
	AUTH="Authorization: Basic $(echo -n $ApiUsername:$ApiPassword | base64)"

	echo "####################################################################"
	echo "## Command (SSH) to MCIS "
	echo "####################################################################"

	CSP=${1}
	REGION=${2:-1}
	POSTFIX=${3:-developer}

	source ../common-functions.sh
	getCloudIndex $CSP


	MCISID=${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX}

	if [ "${INDEX}" == "0" ]; then
		MCISPREFIX=avengers
		MCISID=${MCISPREFIX}-${POSTFIX}
	fi

	curl -H "${AUTH}" -sX POST http://$TumblebugServer/tumblebug/ns/$NS_ID/cmd/mcis/$MCISID -H 'Content-Type: application/json' -d \
		'{
			"command": "wget https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/master/assets/scripts/setcbsp.sh -O ~/setcbtb.sh; chmod +x ~/setcbtb.sh; ~/setcbtb.sh"
		}' | json_pp #|| return 1
#}

#deploy_nginx_to_mcis