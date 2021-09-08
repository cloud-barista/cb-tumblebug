#!/bin/bash

function clean_mcis_sequence() {
	local CSP=$1
	local REGION=$2
	local POSTFIX=$3
	local TestSetFile=$4

	echo '## 8. MCIS: Terminate'
	OUTPUT=$(../8.mcis/terminate-mcis.sh -c $CSP -r $REGION -n $POSTFIX -f $TestSetFile)
	echo "${OUTPUT}"
	OUTPUT1=$(echo "${OUTPUT}" | grep -c 'No VM to terminate')
	OUTPUT2=$(echo "${OUTPUT}" | grep -c 'Terminate is not allowed')
	OUTPUT3=$(echo "${OUTPUT}" | grep -c 'does not exist')

	if [ "${OUTPUT1}" != 1 ] && [ "${OUTPUT2}" != 1 ] && [ "${OUTPUT3}" != 1 ]; then
		echo "============== sleep 30 before delete MCIS obj"
		dozing 30
	fi

	../8.mcis/delete-mcis.sh -c $CSP -r $REGION -n $POSTFIX -f $TestSetFile
}

SECONDS=0

echo "####################################################################"
echo "## Remove MCIS only"
echo "####################################################################"

source ../init.sh

if [ "${INDEX}" == "0" ]; then
	echo "[Parallel execution for all CSP regions]"
else
	echo "[Single execution for a CSP region]"
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
