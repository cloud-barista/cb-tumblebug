#!/bin/bash

function clean_infra_sequence() {
	local CSP=$1
	local REGION=$2
	local POSTFIX=$3
	local TestSetFile=$4

	# echo '## 8. Infra: Refine first (remove failed VMs)'
	# OUTPUT=$(../8.infra/refine-infra.sh -c $CSP -r $REGION -n $POSTFIX -f $TestSetFile)

	# echo '## 8. Infra: Terminate'
	# OUTPUT=$(../8.infra/terminate-infra.sh -c $CSP -r $REGION -n $POSTFIX -f $TestSetFile)
	# echo "${OUTPUT}"
	# OUTPUT1=$(echo "${OUTPUT}" | grep -c 'No VM to terminate')
	# OUTPUT2=$(echo "${OUTPUT}" | grep -c 'Terminate is not allowed')
	# OUTPUT3=$(echo "${OUTPUT}" | grep -c 'does not exist')

	# if [ "${OUTPUT1}" != 1 ] && [ "${OUTPUT2}" != 1 ] && [ "${OUTPUT3}" != 1 ]; then
	# 	echo "============== sleep 30 before delete Infra obj"
	# 	dozing 30
	# fi

	../8.infra/delete-infra.sh -c $CSP -r $REGION -n $POSTFIX -f $TestSetFile -x terminate
}

SECONDS=0

echo "####################################################################"
echo "## Remove Infra only"
echo "####################################################################"

source ../init.sh

if [ "${INDEX}" == "0" ]; then
	echo "[Parallel execution for all CSP regions]"
else
	echo "[Single execution for a CSP region]"
fi
clean_infra_sequence $CSP $REGION $POSTFIX $TestSetFile

echo -e "${BOLD}"
echo "[Cleaning related commands in history file executionStatus]"
echo -e ""
echo -e "${NC}${BLUE}- Removing  (Infra) ${CSP} ${REGION} ${POSTFIX} ${TestSetFile}"
echo -e "${NC}"
sed -i "/(Infra) ${CSP} ${REGION} ${POSTFIX} ${TestSetFile//\//\\/}/d" ./executionStatus
echo ""
echo "[Executed Command List]"
cat ./executionStatus
cp ./executionStatus ./executionStatus.back
echo ""

duration=$SECONDS

printElapsed $@
