#!/bin/bash


# Function for individual CSP test
function test_sequence()
{
	local CSP=$1
	local REGION=$2
	local POSTFIX=$3
	local NUMVM=$4
	local CMDPATH=$5

	../1.configureSpider/register-cloud.sh $CSP $REGION $POSTFIX
	../2.configureTumblebug/create-ns.sh $CSP $REGION $POSTFIX
	../3.vNet/create-vNet.sh $CSP $REGION $POSTFIX
	dozing 10
	if [ "${CSP}" == "gcp" ]; then
		echo "[Test for GCP needs more preparation time]"
		dozing 20
	fi
	../4.securityGroup/create-securityGroup.sh $CSP $REGION $POSTFIX
	dozing 10
	../5.sshKey/create-sshKey.sh $CSP $REGION $POSTFIX
	../6.image/registerImageWithId.sh $CSP $REGION $POSTFIX
	../7.spec/register-spec.sh $CSP $REGION $POSTFIX
	#../8.mcis/create-mcis.sh $CSP $REGION $POSTFIX $NUMVM
	#dozing 1
	#../8.mcis/status-mcis.sh $CSP $REGION $POSTFIX

	_self=$CMDPATH

	echo ""
	echo "[Logging to notify latest command history]"
	echo "[CMD] ${_self} ${CSP} ${REGION} ${POSTFIX} ${NUMVM}" >> ./executionStatus
	echo ""
	echo "[Executed Command List]"
	cat  ./executionStatus
	echo ""
}

# Functions for all CSP test
function test_sequence_allcsp_mcir()
{
	local CSP=$1
	local REGION=$2
	local POSTFIX=$3
	local CMDPATH=$4

	../1.configureSpider/register-cloud.sh $CSP $REGION $POSTFIX
	../2.configureTumblebug/create-ns.sh $CSP $REGION $POSTFIX
	../3.vNet/create-vNet.sh $CSP $REGION $POSTFIX
	dozing 10
	if [ "${CSP}" == "gcp" ]; then
		echo "[Test for GCP needs more preparation time]"
		dozing 20
	fi
	../4.securityGroup/create-securityGroup.sh $CSP $REGION $POSTFIX
	dozing 10
	../5.sshKey/create-sshKey.sh $CSP $REGION $POSTFIX
	../6.image/registerImageWithId.sh $CSP $REGION $POSTFIX
	../7.spec/register-spec.sh $CSP $REGION $POSTFIX

	_self=$CMDPATH

	echo ""
	echo "[Logging to notify latest command history]"
	echo "[CMD] ${_self} ${CSP} ${REGION} ${POSTFIX}" >> ./executionStatus
	echo ""
	echo "[Executed Command List]"
	cat  ./executionStatus
	echo ""

}

function test_sequence_allcsp_mcis()
{
	local CSP=$1
	local REGION=$2
	local POSTFIX=$3
	local MCISPREFIX=$4

	../8.mcis/create-single-vm-mcis.sh $CSP $REGION $POSTFIX $MCISPREFIX
	dozing 1
	../8.mcis/status-mcis.sh $CSP $REGION $POSTFIX $MCISPREFIX

}

function test_sequence_allcsp_mcis_vm()
{
	local CSP=$1
	local REGION=$2
	local POSTFIX=$3
	local NUMVM=$4
	local MCISPREFIX=$5

	../8.mcis/add-vmgroup-to-mcis.sh $CSP $REGION $POSTFIX $NUMVM $MCISPREFIX

}
	SECONDS=0

    FILE=../conf.env
    if [ ! -f "$FILE" ]; then
        echo "$FILE does not exist."
        exit
    fi

	FILE=../credentials.conf
    if [ ! -f "$FILE" ]; then
        echo "$FILE does not exist."
        exit
    fi


	TestSetFile=${5:-../testSet.env}
    
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


	CSP=${1}
	REGION=${2:-1}
	POSTFIX=${3:-developer}
	NUMVM=${4:-3}

	source ../common-functions.sh
	getCloudIndex $CSP

	if [ "${INDEX}" == "0" ]; then
		echo "[Parallel excution for all CSP regions]"

		INDEXX=${NumCSP}
		for ((cspi=1;cspi<=INDEXX;cspi++)); do
			#echo $i
			INDEXY=${NumRegion[$cspi]}
			CSP=${CSPType[$cspi]}
			for ((cspj=1;cspj<=INDEXY;cspj++)); do
				#echo $j
				REGION=$cspj

				echo $CSP
				echo $REGION

				test_sequence_allcsp_mcir $CSP $REGION $POSTFIX ${0##*/} &

			done
			
		done
		wait

		MCISPREFIX=avengers

		
	else
		echo "[Single excution for a CSP region]"

		test_sequence $CSP $REGION $POSTFIX $NUMVM ${0##*/}

	fi

	duration=$SECONDS
	echo "$(($duration / 60)) minutes and $(($duration % 60)) seconds elapsed."
#}

#testAll
