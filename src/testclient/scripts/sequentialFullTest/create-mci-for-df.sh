#!/bin/bash

# Function for individual CSP test
function test_sequence()
{
	local CSP=$1
	local REGION=$2
	local POSTFIX=$3
	local TestSetFile=$4
	local CMDPATH=$5

	../1.configureSpider/register-cloud.sh -c $CSP -r $REGION -n $POSTFIX -f $TestSetFile
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
	../8.mci/create-mci-no-agent.sh -c $CSP -r $REGION -n $POSTFIX -f $TestSetFile
	dozing 1
	../8.mci/status-mci.sh -c $CSP -r $REGION -n $POSTFIX -f $TestSetFile

	_self=$CMDPATH

	echo ""
	echo "[Logging to notify latest command history]"
	echo "[CMD] (ResourceS(${SECONDS}s)) ${_self} (MCI) ${CSP} ${REGION} ${POSTFIX} ${TestSetFile}" >>./executionStatus
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
	echo "## Create MCI from Zero Base"
	echo "####################################################################"


	CSP=${1}
	REGION=${2:-1}
	POSTFIX=${3:-developer}

	source ../common-functions.sh
	getCloudIndex $CSP

	echo "[Single execution for a CSP region]"
	test_sequence $CSP $REGION $POSTFIX $TestSetFile ${0##*/}

	echo "[Deploy CB-Dragonfly Docker]"
	dozing 60
	./deploy-dragonfly-docker.sh -c $CSP -r $REGION -n $POSTFIX -f $TestSetFile

#}

#testAll
