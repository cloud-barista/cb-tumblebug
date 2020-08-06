#!/bin/bash

function dozing()
{
	duration=$1
	printf "Dozing for %s : " $duration
	for (( i=1; i<=$duration; i++ ))
	do
		printf "%s " $i
		sleep 1
	done
	echo "(Back to work)"
}

function test_sequence()
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
		../6.image/registerImageWithInfo.sh $CSP $REGION $POSTFIX
		../7.spec/register-spec.sh $CSP $REGION $POSTFIX
		../8.mcis/create-mcis.sh $CSP $REGION $POSTFIX
		dozing 1
		../8.mcis/status-mcis.sh $CSP $REGION $POSTFIX

		_self=$CMDPATH

		echo ""
		echo "[Logging to notify latest command history]"
		echo "[CMD] ${_self} ${CSP} ${REGION} ${POSTFIX}" >> ./executionStatus
		echo ""
		echo "[Executed Command List]"
		cat  ./executionStatus
		echo ""

}


#function testAll() {
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

	source ../conf.env
	AUTH="Authorization: Basic $(echo -n $ApiUsername:$ApiPassword | base64)"
	source ../credentials.conf

	echo "####################################################################"
	echo "## Create MCIS from Zero Base"
	echo "####################################################################"


	CSP=${1}
	REGION=${2:-1}
	POSTFIX=${3:-developer}
	if [ "${CSP}" == "all" ]; then
		echo "[Test for all CSP regions (AWS, Azure, GCP, Alibaba, ...)]"
		CSP="aws"
		INDEX=0
	elif [ "${CSP}" == "aws" ]; then
		echo "[Test for AWS]"
		INDEX=1
	elif [ "${CSP}" == "azure" ]; then
		echo "[Test for Azure]"
		INDEX=2
	elif [ "${CSP}" == "gcp" ]; then
		echo "[Test for GCP]"
		INDEX=3
	elif [ "${CSP}" == "alibaba" ]; then
		echo "[Test for Alibaba]"
		INDEX=4
	else
		echo "[No acceptable argument was provided (all, aws, azure, gcp, alibaba, ...). Default: Test for AWS]"
		CSP="aws"
		INDEX=1
	fi

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

				test_sequence $CSP $REGION $POSTFIX ${0##*/}

			done
		done
		
	else
		echo "[Single excution for a CSP region]"

		test_sequence $CSP $REGION $POSTFIX ${0##*/}

	fi

#}

#testAll