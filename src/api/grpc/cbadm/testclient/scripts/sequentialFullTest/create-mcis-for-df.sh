#!/bin/bash

# Function for individual CSP test
function test_sequence()
{
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
	../8.mcis/create-mcis-no-agent.sh $CSP $REGION $POSTFIX $TestSetFile
	dozing 1
	../8.mcis/status-mcis.sh $CSP $REGION $POSTFIX $TestSetFile

	_self=$CMDPATH

	echo ""
	echo "[Logging to notify latest command history]"
	echo "[CMD] (MCIRS(${SECONDS}s)) ${_self} (MCIS) ${CSP} ${REGION} ${POSTFIX} ${TestSetFile}" >>./executionStatus
	echo ""
	echo "[Executed Command List]"
	cat  ./executionStatus
	cp ./executionStatus ./executionStatus.back
	echo ""
}




	FILE=../../../../conf/credentials.conf
    if [ ! -f "$FILE" ]; then
        echo "$FILE does not exist."
        exit
    fi

	TestSetFile=${4:-../testSet.env}
    if [ ! -f "$TestSetFile" ]; then
        echo "$TestSetFile does not exist."
        exit
    fi
	source $TestSetFile
    source ../conf.env
		source ../../../../conf/credentials.conf

	echo "####################################################################"
	echo "## Create MCIS from Zero Base"
	echo "####################################################################"


	CSP=${1}
	REGION=${2:-1}
	POSTFIX=${3:-developer}

	source ../common-functions.sh
	getCloudIndex $CSP

	echo "[Single excution for a CSP region]"
	test_sequence $CSP $REGION $POSTFIX $TestSetFile ${0##*/}

	echo "[Deploy CB-Dragonfly Docker]"
	dozing 60
	./deploy-dragonfly-docker.sh $CSP $REGION $POSTFIX $TestSetFile

#}

#testAll
