#!/bin/bash

# Function for individual CSP test
function test_sequence() {
	local CSP=$1
	local REGION=$2
	local POSTFIX=$3
	local TestSetFile=$4
	local NUMVM=$5

	local CMDPATH=$6

	../1.configureSpider/register-cloud.sh $CSP $REGION $POSTFIX $TestSetFile
	../2.configureTumblebug/create-ns.sh $CSP $REGION $POSTFIX $TestSetFile
	../3.vNet/create-vNet.sh $CSP $REGION $POSTFIX $TestSetFile
	dozing 10
	if [ "${CSP}" == "gcp" ]; then
		echo "[Test for GCP needs more preparation time]"
		dozing 20
	fi
	../4.securityGroup/create-securityGroup.sh $CSP $REGION $POSTFIX $TestSetFile
	dozing 10
	../5.sshKey/create-sshKey.sh $CSP $REGION $POSTFIX $TestSetFile
	../6.image/registerImageWithId.sh $CSP $REGION $POSTFIX $TestSetFile
	../7.spec/register-spec.sh $CSP $REGION $POSTFIX $TestSetFile
	# ../8.mcis/create-mcis.sh $CSP $REGION $POSTFIX $NUMVM $TestSetFile
	# dozing 1
	# ../8.mcis/status-mcis.sh $CSP $REGION $POSTFIX $TestSetFile

	_self=$CMDPATH

	echo ""
	echo "[Logging to notify latest command history]"
	echo "[MCIR:${MCIRRegionName}] ${_self} (MCIR) ${CSP} ${REGION} ${POSTFIX} ${TestSetFile}" >>./executionStatus
	echo ""
	echo "[Executed Command List]"
	#cat ./executionStatus
	cp ./executionStatus ./executionStatus.back
	echo ""
}

# Functions for all CSP test
function test_sequence_allcsp_mcir() {
	local CSP=$1
	local REGION=$2
	local POSTFIX=$3
	local TestSetFile=$4
	local CMDPATH=$5

	../1.configureSpider/register-cloud.sh $CSP $REGION $POSTFIX $TestSetFile
	../2.configureTumblebug/create-ns.sh $CSP $REGION $POSTFIX $TestSetFile
	../3.vNet/create-vNet.sh $CSP $REGION $POSTFIX $TestSetFile
	dozing 10
	if [ "${CSP}" == "gcp" ]; then
		echo "[Test for GCP needs more preparation time]"
		dozing 20
	fi
	../4.securityGroup/create-securityGroup.sh $CSP $REGION $POSTFIX $TestSetFile
	dozing 10
	../5.sshKey/create-sshKey.sh $CSP $REGION $POSTFIX $TestSetFile
	../6.image/registerImageWithId.sh $CSP $REGION $POSTFIX $TestSetFile
	../7.spec/register-spec.sh $CSP $REGION $POSTFIX $TestSetFile

	_self=$CMDPATH

	echo ""
	echo "[Logging to notify latest command history]"
	echo "[MCIR:${MCIRRegionName}] ${_self} (MCIR) ${CSP} ${REGION} ${POSTFIX} ${TestSetFile}" >>./executionStatus
	echo ""
	echo "[Executed Command List]"
	cat ./executionStatus
	cp ./executionStatus ./executionStatus.back
	echo ""

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
source ../credentials.conf
AUTH="Authorization: Basic $(echo -n $ApiUsername:$ApiPassword | base64)"

echo "####################################################################"
echo "## Create MCIS from Zero Base"
echo "####################################################################"

# CSP=${1}
# REGION=${2:-1}
# POSTFIX=${3:-developer}
NUMVM=${5:-1}

source ../common-functions.sh
readParameters "$@"
getCloudIndex $CSP

if [ "${INDEX}" == "0" ]; then
	echo "[Parallel excution for all CSP regions]"

	INDEXX=${NumCSP}
	for ((cspi = 1; cspi <= INDEXX; cspi++)); do
		#echo $i
		INDEXY=${NumRegion[$cspi]}
		CSP=${CSPType[$cspi]}
		for ((cspj = 1; cspj <= INDEXY; cspj++)); do
			#echo $j
			REGION=$cspj
			#echo $CSP
			#echo $REGION
			MCIRRegionName=${RegionName[$cspi,$cspj]}
			echo "- Create MCIR in ${MCIRRegionName}"		

			test_sequence_allcsp_mcir $CSP $REGION $POSTFIX $TestSetFile ${0##*/} &
			# dozing 1

		done

	done
	wait


	MCISID=${MCISPREFIX}-${POSTFIX}


else
	echo ""
	TOTALVM=$((1 * 1 * NUMVM))
	echo "[Create MCIS] VMs($TOTALVM) = Cloud(1) * Region(1) * VMgroup($NUMVM)"
	MCIRRegionName=${CONN_CONFIG[$INDEX,$REGION]}

	test_sequence $CSP $REGION $POSTFIX $TestSetFile $NUMVM ${0##*/}

fi

duration=$SECONDS
echo "[ElapsedTime] $(($duration / 60)):$(($duration % 60)) (min:sec) $duration (sec) / [Command] $0 "

