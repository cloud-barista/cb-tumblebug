#!/bin/bash

# Function for individual CSP test
function test_sequence()
{
	local CSP=$1
	local REGION=$2
	local POSTFIX=$3
	local NUMVM=$4
	local TestSetFile=$5
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
	echo "[CMD] ${_self} ${CSP} ${REGION} ${POSTFIX} ${TestSetFile}" >> ./executionStatus
	echo ""
	echo "[Executed Command List]"
	cat  ./executionStatus
	echo ""

}

# function test_sequence_allcsp_mcis()
# {
# 	local CSP=$1
# 	local REGION=$2
# 	local POSTFIX=$3
# 	local NUMVM=$4
# 	local MCISPREFIX=$5
# 	local TestSetFile=$6
# 	local CMDPATH=$7

# 	_self=$CMDPATH

# 	../8.mcis/create-single-vm-mcis.sh $CSP $REGION $POSTFIX $NUMVM $MCISPREFIX $TestSetFile
# 	#dozing 1
# 	#../8.mcis/status-mcis.sh $CSP $REGION $POSTFIX $TestSetFile $MCISPREFIX
# 	echo ""
# 	echo "[Logging to notify latest command history]"
# 	echo "[CMD] ${_self} all 1 ${POSTFIX} ${TestSetFile}" >> ./executionStatus
# 	echo ""
# 	echo "[Executed Command List]"
# 	cat  ./executionStatus
# 	echo ""

# }

# function test_sequence_allcsp_mcis_vm()
# {
# 	local CSP=$1
# 	local REGION=$2
# 	local POSTFIX=$3
# 	local NUMVM=$4
# 	local MCISPREFIX=$5
# 	local TestSetFile=$6

# 	../8.mcis/add-vmgroup-to-mcis.sh $CSP $REGION $POSTFIX $NUMVM $MCISPREFIX $TestSetFile

# }
	SECONDS=0

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
				#echo $CSP
				#echo $REGION
				echo "- Create MCIR in ${CONN_CONFIG[$cspi,$REGION]}"

				test_sequence_allcsp_mcir $CSP $REGION $POSTFIX $TestSetFile ${0##*/} &
				dozing 1

			done
			
		done
		wait

		MCISPREFIX=avengers
		MCISID=${MCISPREFIX}-${POSTFIX}

		#test_sequence_allcsp_mcis ${CSPType[1]} 1 $POSTFIX $MCISPREFIX

		# for ((cspi=1;cspi<=INDEXX;cspi++)); do
		# 	#echo $i
		# 	INDEXY=${NumRegion[$cspi]}
		# 	CSP=${CSPType[$cspi]}
		# 	for ((cspj=1;cspj<=INDEXY;cspj++)); do
		# 		#echo $j
		# 		REGION=$cspj

		# 		#echo $CSP
		# 		#echo $REGION
		# 		TOTALVM=$((INDEXX * INDEXY * NUMVM))
		# 		echo "[Create MCIS] VMs($TOTALVM) = Cloud($INDEXX) * Region($INDEXY) * VMgroup($NUMVM)"
		# 		echo "- Create VM in ${CONN_CONFIG[$cspi,$REGION]}"
				

		# 		if [ "${cspi}" -eq 1 ] && [ "${cspj}" -eq 1 ]; then
		# 			echo ""
		# 			echo "[Create a MCIS object with a VM]"
		# 			test_sequence_allcsp_mcis $CSP $REGION $POSTFIX $NUMVM $MCISPREFIX $TestSetFile ${0##*/} &
		# 			# Check MCIS object is created 
		# 			echo ""
		# 			echo "[Waiting for initialization of MCIS:$MCISID (5s)]"
		# 			dozing 5
					
		# 			echo "Checking MCIS object. (upto 3s * 20 trials)" 
		# 			for (( try=1; try<=20; try++ ))
		# 			do
		# 				HTTP_CODE=0
		# 				HTTP_CODE=$(curl -H "${AUTH}" -o /dev/null --write-out "%{http_code}\n" "http://$TumblebugServer/tumblebug/ns/$NS_ID/mcis/${MCISID}" --silent)
		# 				echo "HTTP status for get MCIS object: $HTTP_CODE" 
		# 				if [ ${HTTP_CODE} -ge 200 -a ${HTTP_CODE} -le 204 ]; then
		# 					echo "[$try : MCIS object is READY]"
		# 					break
		# 				else 
		# 					printf "[$try : MCIS object is not ready yet].."
		# 					dozing 3
		# 				fi
		# 			done
		# 		else
		# 			echo ""
		# 			echo "[Create VM and add it into the MCIS in parallel]"
		# 			test_sequence_allcsp_mcis_vm $CSP $REGION $POSTFIX $NUMVM $MCISPREFIX $TestSetFile &
		# 			dozing 1
		# 		fi
		# 	done
			
		# done
		# wait

		# ../8.mcis/status-mcis.sh ${CSPType[1]} 1 $POSTFIX $TestSetFile $MCISPREFIX
		
	else
		echo ""
		TOTALVM=$((1 * 1 * NUMVM))
		echo "[Create MCIS] VMs($TOTALVM) = Cloud(1) * Region(1) * VMgroup($NUMVM)"

		test_sequence $CSP $REGION $POSTFIX $NUMVM $TestSetFile ${0##*/}

	fi

	duration=$SECONDS
	echo "$(($duration / 60)) minutes and $(($duration % 60)) seconds elapsed."

	echo ""
	echo "[Executed Command List]"
	cat  ./executionStatus
	echo ""


