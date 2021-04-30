#!/bin/bash


TestSetFile=${4:-../testSet.env}

FILE=$TestSetFile
if [ ! -f "$FILE" ]; then
	echo "$FILE does not exist."
	exit
fi
source $TestSetFile
source ../conf.env
AUTH="Authorization: Basic $(echo -n $ApiUsername:$ApiPassword | base64)"

echo "####################################################################"
echo "## 8. Get MCIS"
echo "####################################################################"

CSP=${1}
REGION=${2:-1}
POSTFIX=${3:-developer}

source ../common-functions.sh
getCloudIndex $CSP


if [ "${INDEX}" == "0" ]; then
	MCISID=${MCISPREFIX}-${POSTFIX}
else
	MCISID=${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX}
fi

echo "${MCISID}"

curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NSID/mcis/${MCISID} | jq ''


#get_mcis