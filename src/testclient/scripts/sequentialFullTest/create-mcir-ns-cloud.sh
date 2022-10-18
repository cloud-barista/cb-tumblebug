#!/bin/bash

# Function for individual CSP test
function test_sequence() {
	local CSP=$1
	local REGION=$2
	local POSTFIX=$3
	local TestSetFile=$4
	local NUMVM=$5

	local CMDPATH=$6

	# ../1.configureSpider/register-cloud.sh -c $CSP -r $REGION -n $POSTFIX -f $TestSetFile
	../2.configureTumblebug/create-ns.sh -n $POSTFIX -f $TestSetFile
	../3.vNet/create-vNet.sh -c $CSP -r $REGION -n $POSTFIX -f $TestSetFile
	dozing 10
	if [ "${CSP}" == "gcp" ]; then
		echo "[Test for GCP needs more preparation time]"
		dozing 20
	fi
	../4.securityGroup/create-securityGroup.sh -c $CSP -r $REGION -n $POSTFIX -f $TestSetFile
	dozing 10
	../5.sshKey/create-sshKey.sh -c $CSP -r $REGION -n $POSTFIX -f $TestSetFile
	../6.image/registerImageWithId.sh -c $CSP -r $REGION -n $POSTFIX -f $TestSetFile
	../7.spec/register-spec.sh -c $CSP -r $REGION -n $POSTFIX -f $TestSetFile
	# ../8.mcis/create-mcis.sh $CSP $REGION $POSTFIX $NUMVM $TestSetFile
	# dozing 1
	# ../8.mcis/status-mcis.sh -c $CSP -r $REGION -n $POSTFIX -f $TestSetFile

	_self=$CMDPATH

	echo ""
	echo "[Logging to notify latest command history]"
	echo "[MCIR:${MCIRRegionName}(${SECONDS}s)] ${_self} (MCIR) ${CSP} ${REGION} ${POSTFIX} ${TestSetFile}" >>./executionStatus
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

	# ../1.configureSpider/register-cloud.sh -c $CSP -r $REGION -n $POSTFIX -f $TestSetFile
	../2.configureTumblebug/create-ns.sh -n $POSTFIX -f $TestSetFile
	../3.vNet/create-vNet.sh -c $CSP -r $REGION -n $POSTFIX -f $TestSetFile
	dozing 10
	if [ "${CSP}" == "gcp" ]; then
		echo "[Test for GCP needs more preparation time]"
		dozing 20
	fi
	../4.securityGroup/create-securityGroup.sh -c $CSP -r $REGION -n $POSTFIX -f $TestSetFile
	dozing 10
	../5.sshKey/create-sshKey.sh -c $CSP -r $REGION -n $POSTFIX -f $TestSetFile
	../6.image/registerImageWithId.sh -c $CSP -r $REGION -n $POSTFIX -f $TestSetFile
	../7.spec/register-spec.sh -c $CSP -r $REGION -n $POSTFIX -f $TestSetFile

	_self=$CMDPATH

	echo ""
	echo "[Logging to notify latest command history]"
	echo "[MCIR:${MCIRRegionName}(${SECONDS}s)] ${_self} (MCIR) ${CSP} ${REGION} ${POSTFIX} ${TestSetFile}" >>./executionStatus
	echo ""
	echo "[Executed Command List]"
	cat ./executionStatus
	cp ./executionStatus ./executionStatus.back
	echo ""

	duration=$SECONDS
	printElapsed $CSP $REGION $POSTFIX $TestSetFile $MCIRRegionName

}
SECONDS=0

echo "####################################################################"
echo "## Create mcir-ns-cloud from Zero Base"
echo "####################################################################"

source ../init.sh

NUMVM=${OPTION01:-1}


if [ "${INDEX}" == "0" ]; then
	echo "[Parallel execution for all CSP regions]"

	INDEXX=${NumCSP}
	for ((cspi = 1; cspi <= INDEXX; cspi++)); do
		echo $i
		INDEXY=${NumRegion[$cspi]}
		CSP=${CSPType[$cspi]}
		for ((cspj = 1; cspj <= INDEXY; cspj++)); do
			echo $j
			REGION=$cspj
			echo $CSP
			echo $REGION
			echo ${RegionName[1,1]}
			MCIRRegionName=${RegionName[$cspi,$cspj]}
			echo "- Create MCIR in ${MCIRRegionName}"		

			test_sequence_allcsp_mcir $CSP $REGION $POSTFIX $TestSetFile ${0##*/} &
			# dozing 1

		done

	done
	wait


	MCISID=${POSTFIX}


else
	echo ""
	TOTALVM=$((1 * 1 * NUMVM))
	echo "[Create MCIS] VMs($TOTALVM) = Cloud(1) * Region(1) * subGroup($NUMVM)"
	MCIRRegionName=${CONN_CONFIG[$INDEX,$REGION]}

	test_sequence $CSP $REGION $POSTFIX $TestSetFile $NUMVM ${0##*/}

fi

duration=$SECONDS

printElapsed $@

