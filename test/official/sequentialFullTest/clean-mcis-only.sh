#!/bin/bash

function clean_sequence() {
	local CSP=$1
	local REGION=$2
	local POSTFIX=$3
	local TestSetFile=$4
	local CMDPATH=$5

	echo '## 8. MCIS: Terminate'
	OUTPUT=$(../8.mcis/just-terminate-mcis.sh $CSP $REGION $POSTFIX $TestSetFile)
	echo "${OUTPUT}"
	OUTPUT1=$(echo "${OUTPUT}" | grep -c 'No VM to terminate')
	OUTPUT2=$(echo "${OUTPUT}" | grep -c 'Terminate is not allowed')
	OUTPUT3=$(echo "${OUTPUT}" | grep -c 'does not exist')

	if [ "${OUTPUT1}" != 1 ] && [ "${OUTPUT2}" != 1 ] && [ "${OUTPUT3}" != 1 ]; then
		echo "============== sleep 20 before delete MCIS obj"
		dozing 20
	fi

	#../8.mcis/status-mcis.sh $CSP $REGION $POSTFIX $TestSetFile
	../8.mcis/terminate-and-delete-mcis.sh $CSP $REGION $POSTFIX $TestSetFile
	# ../7.spec/unregister-spec.sh $CSP $REGION $POSTFIX $TestSetFile
	# ../6.image/unregister-image.sh $CSP $REGION $POSTFIX $TestSetFile

	# # echo '## 5. sshKey: Delete'
	# # OUTPUT=$(../5.sshKey/delete-sshKey.sh $CSP $REGION $POSTFIX)
	# # echo "${OUTPUT}"
	# # OUTPUT=$(echo "${OUTPUT}" | grep -c 'does not exist')
	# # echo "${OUTPUT}"
	# # if [ "${OUTPUT}" != 1 ]; then
	# # 	echo "============== sleep 10 after delete-sshKey"
	# # 	dozing 5
	# # fi

	# echo '## 5. sshKey: Delete'
	# OUTPUT=$(../5.sshKey/delete-sshKey.sh $CSP $REGION $POSTFIX $TestSetFile)
	# echo "${OUTPUT}"
	# OUTPUT=$(echo "${OUTPUT}" | grep -c -e 'Error' -e 'error' -e 'dependency' -e 'dependent' -e 'DependencyViolation')
	# echo "${OUTPUT}"
	# if [ "${OUTPUT}" != 0 ]; then

	# 	echo "Retry sshKey: Delete 20 times"
	# 	for (( c=1; c<=40; c++ ))
	# 	do
	# 		echo "Trial: ${c}. Sleep 5 before retry sshKey: Delete"
	# 		dozing 5
	# 		# retry sshKey: Delete
	# 		OUTPUT2=$(../5.sshKey/delete-sshKey.sh $CSP $REGION $POSTFIX $TestSetFile)
	# 		echo "${OUTPUT2}"
	# 		OUTPUT2=$(echo "${OUTPUT2}" | grep -c -e 'Error' -e 'error' -e 'dependency' -e 'dependent' -e 'DependencyViolation')
	# 		echo "${OUTPUT2}"

	# 		if [ "${OUTPUT2}" == 0 ]; then
	# 			echo "OK. sshKey: Delete complete"
	# 			break
	# 		fi

	# 		if [ "${c}" == 20 ] && [ "${OUTPUT2}" == 1 ]
	# 		then
	# 			echo "Problem in sshKey: Delete. Exit without unregister-cloud."
	# 			exit
	# 		fi
	# 	done

	# fi

	# # echo '## 4. SecurityGroup: Delete'
	# # OUTPUT=$(../4.securityGroup/delete-securityGroup.sh $CSP $REGION $POSTFIX)
	# # echo "${OUTPUT}"
	# # OUTPUT=$(echo "${OUTPUT}" | grep -c 'does not exist')
	# # echo "${OUTPUT}"
	# # if [ "${OUTPUT}" != 1 ]; then
	# # 	echo "============== sleep 10 after delete-securityGroup"
	# # 	dozing 5
	# # fi

	# echo '## 4. SecurityGroup: Delete'
	# OUTPUT=$(../4.securityGroup/delete-securityGroup.sh $CSP $REGION $POSTFIX $TestSetFile)
	# echo "${OUTPUT}"
	# OUTPUT=$(echo "${OUTPUT}" | grep -c -e 'Error' -e 'error' -e 'dependency' -e 'dependent' -e 'DependencyViolation')
	# echo "${OUTPUT}"
	# if [ "${OUTPUT}" != 0 ]; then

	# 	echo "Retry SecurityGroup: Delete 30 times"
	# 	for (( c=1; c<=50; c++ ))
	# 	do
	# 		echo "Trial: ${c}. Sleep 5 before retry SecurityGroup: Delete"
	# 		dozing 5
	# 		# retry SecurityGroup: Delete
	# 		OUTPUT2=$(../4.securityGroup/delete-securityGroup.sh $CSP $REGION $POSTFIX $TestSetFile)
	# 		echo "${OUTPUT2}"
	# 		OUTPUT2=$(echo "${OUTPUT2}" | grep -c -e 'Error' -e 'error' -e 'dependency' -e 'dependent' -e 'DependencyViolation')
	# 		echo "${OUTPUT2}"

	# 		if [ "${OUTPUT2}" == 0 ]; then
	# 			echo "OK. SecurityGroup: Delete complete"
	# 			break
	# 		fi

	# 		if [ "${c}" == 30 ] && [ "${OUTPUT2}" == 1 ]
	# 		then
	# 			echo "Problem in SecurityGroup: Delete. Exit without unregister-cloud."
	# 			exit
	# 		fi
	# 	done

	# fi

	# echo '## 3. vNet: Delete'
	# OUTPUT=$(../3.vNet/delete-vNet.sh $CSP $REGION $POSTFIX $TestSetFile)
	# echo "${OUTPUT}"
	# OUTPUT=$(echo "${OUTPUT}" | grep -c -e 'Error' -e 'error' -e 'dependency' -e 'dependent' -e 'DependencyViolation')
	# echo "${OUTPUT}"
	# if [ "${OUTPUT}" != 0 ]; then

	# 	echo "Retry delete-vNet 40 times"
	# 	for (( c=1; c<=60; c++ ))
	# 	do
	# 		echo "Trial: ${c}. Sleep 5 before retry delete-vNet"
	# 		dozing 5
	# 		# retry delete-vNet
	# 		OUTPUT2=$(../3.vNet/delete-vNet.sh $CSP $REGION $POSTFIX $TestSetFile)
	# 		echo "${OUTPUT2}"
	# 		OUTPUT2=$(echo "${OUTPUT2}" | grep -c -e 'Error' -e 'error' -e 'dependency' -e 'dependent' -e 'DependencyViolation')
	# 		echo "${OUTPUT2}"

	# 		if [ "${OUTPUT2}" == 0 ]; then
	# 			echo "OK. delete-vNet complete"
	# 			break
	# 		fi

	# 		if [ "${c}" == 20 ] && [ "${OUTPUT2}" == 1 ]
	# 		then
	# 			echo "Problem in delete-vNet. Exit without unregister-cloud."
	# 			exit
	# 		fi
	# 	done

	# fi

	# #../2.configureTumblebug/delete-ns.sh $CSP $REGION $POSTFIX

	# CNT=$(grep -c "${CSP}" ./executionStatus)
	# if [ "${CNT}" -ge 2 ]; then
	# 	../1.configureSpider/unregister-cloud.sh $CSP $REGION $POSTFIX leave $TestSetFile
	# else
	# 	echo "[No dependancy, this CSP can be removed.]"
	# 	../1.configureSpider/unregister-cloud.sh $CSP $REGION $POSTFIX doit $TestSetFile
	# fi

	#_self="${0##*/}"

	echo ""
	echo "[Cleaning related commands in history file executionStatus]"
	sed -i "/${CSP} ${REGION} ${POSTFIX} ${TestSetFile//\//\\/}/d" ./executionStatus
	echo ""
	echo "[Executed Command List]"
	cat ./executionStatus
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
AUTH="Authorization: Basic $(echo -n $ApiUsername:$ApiPassword | base64)"
source ../credentials.conf

echo "####################################################################"
echo "## Remove MCIS"
echo "####################################################################"

CSP=${1}
REGION=${2:-1}
POSTFIX=${3:-developer}

source ../common-functions.sh
getCloudIndex $CSP

if [ "${INDEX}" == "0" ]; then
	echo "[Parallel excution for all CSP regions]"

	# MCISPREFIX=avengers

	OUTPUT=$(../8.mcis/just-terminate-mcis.sh $CSP $REGION $POSTFIX $TestSetFile $MCISPREFIX)
	echo "${OUTPUT}"
	OUTPUT1=$(echo "${OUTPUT}" | grep -c 'No VM to terminate')
	OUTPUT2=$(echo "${OUTPUT}" | grep -c 'Terminate is not allowed')
	OUTPUT3=$(echo "${OUTPUT}" | grep -c 'does not exist')

	if [ "${OUTPUT1}" != 1 ] && [ "${OUTPUT2}" != 1 ] && [ "${OUTPUT3}" != 1 ]; then
		echo "============== sleep 20 before delete MCIS obj"
		dozing 20
	fi
	../8.mcis/terminate-and-delete-mcis.sh $CSP $REGION $POSTFIX $TestSetFile $MCISPREFIX

	# INDEXX=${NumCSP}
	# for ((cspi=1;cspi<=INDEXX;cspi++)); do
	# 	#echo $i
	# 	INDEXY=${NumRegion[$cspi]}
	# 	CSP=${CSPType[$cspi]}
	# 	for ((cspj=1;cspj<=INDEXY;cspj++)); do
	# 		#echo $j
	# 		REGION=$cspj

	# 		echo $CSP
	# 		echo $REGION

	# 		clean_sequence $CSP $REGION $POSTFIX $TestSetFile ${0##*/} &
	# 		dozing 2
	# 	done
	# done
	# wait

	echo ""
	echo "[Cleaning related commands in history file executionStatus]"
	sed -i "/all 1 ${POSTFIX} ${TestSetFile//\//\\/}/d" ./executionStatus
	echo ""
	echo "[Executed Command List]"
	cat ./executionStatus
	echo ""

else
	echo "[Single excution for a CSP region]"

	clean_sequence $CSP $REGION $POSTFIX $TestSetFile ${0##*/}

fi

duration=$SECONDS
echo "$(($duration / 60)) minutes and $(($duration % 60)) seconds elapsed."
#}

#cleanAll
