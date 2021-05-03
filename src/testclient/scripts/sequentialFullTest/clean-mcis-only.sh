#!/bin/bash

function clean_mcis_sequence() {
	local CSP=$1
	local REGION=$2
	local POSTFIX=$3
	local TestSetFile=$4

	echo '## 8. MCIS: Terminate'
	OUTPUT=$(../8.mcis/just-terminate-mcis.sh $CSP $REGION $POSTFIX $TestSetFile)
	echo "${OUTPUT}"
	OUTPUT1=$(echo "${OUTPUT}" | grep -c 'No VM to terminate')
	OUTPUT2=$(echo "${OUTPUT}" | grep -c 'Terminate is not allowed')
	OUTPUT3=$(echo "${OUTPUT}" | grep -c 'does not exist')

	if [ "${OUTPUT1}" != 1 ] && [ "${OUTPUT2}" != 1 ] && [ "${OUTPUT3}" != 1 ]; then
		echo "============== sleep 30 before delete MCIS obj"
		dozing 30
	fi

	../8.mcis/terminate-and-delete-mcis.sh $CSP $REGION $POSTFIX $TestSetFile
}

SECONDS=0

FILE=../credentials.conf
if [ ! -f "$FILE" ]; then
	echo "$FILE does not exist."
	exit
fi

TestSetFile=${4:-../testSet.env}

FILE=$TestSetFile
if [ ! -f "$FILE" ]; then
	echo "$FILE does not exist."
	exit
fi
source $TestSetFile
source ../conf.env
AUTH="Authorization: Basic $(echo -n $ApiUsername:$ApiPassword | base64)"
source ../credentials.conf

echo "####################################################################"
echo "## Remove MCIS only"
echo "####################################################################"

CSP=${1}
REGION=${2:-1}
POSTFIX=${3:-developer}

source ../common-functions.sh
getCloudIndex $CSP

if [ "${INDEX}" == "0" ]; then
	echo "[Parallel excution for all CSP regions]"
else
	echo "[Single excution for a CSP region]"
fi
clean_mcis_sequence $CSP $REGION $POSTFIX $TestSetFile

echo ""
echo "[Cleaning related commands in history file executionStatus]"
echo "Remove (MCIS) ${CSP} ${REGION} ${POSTFIX} ${TestSetFile}"
sed -i "/(MCIS) ${CSP} ${REGION} ${POSTFIX} ${TestSetFile//\//\\/}/d" ./executionStatus
echo ""
echo "[Executed Command List]"
cat ./executionStatus
cp ./executionStatus ./executionStatus.back
echo ""

duration=$SECONDS

printElapsed $@
