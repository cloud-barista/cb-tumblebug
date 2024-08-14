#!/bin/bash

# Function for individual CSP test
function test_sequence_change_k8snodegroup_autoscale_size() {
	local CSP=$1
	local REGION=$2
	local POSTFIX=$3
	local TestSetFile=$4
	local NUMVM=$5 # as DesiredNodeSize
	local CMDPATH=$6

	../13.k8scluster/change-k8snodegroup-autoscalesize.sh -c $CSP -r $REGION -n $POSTFIX -f $TestSetFile -x $NUMVM 
	dozing 1
	# FIXME ../13.k8scluster/status-k8scluster.sh -c $CSP -r $REGION -n $POSTFIX -f $TestSetFile
	#../13.k8scluster/get-k8scluster.sh -c $CSP -r $REGION -n $POSTFIX -f $TestSetFile

	_self=$CMDPATH

	K8SCLUSTERID=${K8SCLUSTERID_PREFIX}${cspi}${cspj}
	echo ""
	echo "[Logging to notify latest command history]"
	echo "[K8SCLUSTER:${K8SCLUSTERID}(${SECONDS}s)] ${_self} (K8sCluster) ${CSP} ${REGION} ${POSTFIX} ${TestSetFile} ${DesiredNodeSize} ${MinNodeSize} ${MaxNodeSize}" >>./executionStatus
	echo ""
	echo "[Executed Command List]"
	#cat ./executionStatus
	cp ./executionStatus ./executionStatus.back
	echo ""
}


function test_sequence_change_k8snodegroup_autoscale_size_allcsp() {
	local CSP=$1
	local REGION=$2
	local POSTFIX=$3
	local TestSetFile=$4
	local NUMVM=$5 # as DesiredNodeSize
	local CMDPATH=$6

	_self=$CMDPATH

	../13.k8scluster/change-k8snodegroup-autoscalesize.sh -c $CSP -r $REGION -n $POSTFIX -f $TestSetFile -x $NUMVM
	#dozing 1
	#../8.mci/status-mci.sh -c $CSP -r $REGION -n $POSTFIX -f $TestSetFile $MCIPREFIX
	#../13.k8scluster/get-k8scluster.sh -c $CSP -r $REGION -n $POSTFIX -f $TestSetFile

	echo ""
	echo "[Logging to notify latest command history]"
	echo "[K8SCLUSTER:${K8SCLUSTERID}(${SECONDS}s+More)] ${_self} (K8SCLUSTER) all 1 ${POSTFIX} ${TestSetFile}" >>./executionStatus
	echo ""
	echo "[Executed Command List]"
	#cat ./executionStatus
	cp ./executionStatus ./executionStatus.back
	echo ""
}

SECONDS=0

echo "####################################################################"
echo "## add NodeGroup from Zero Base"
echo "####################################################################"

source ../init.sh

NUMVM=${OPTION01:-1}


if [ "${INDEX}" == "0" ]; then
	echo "[Parallel execution for all CSP regions]"

	INDEXX=${NumCSP}

	for ((cspi = 1; cspi <= INDEXX; cspi++)); do
		INDEXY=${NumRegion[$cspi]}
		CSP=${CSPType[$cspi]}
		for ((cspj = 1; cspj <= INDEXY; cspj++)); do
			REGION=$cspj

			echo "[set K8SNODEGROUP] K8SCLUSTERs($((INDEXX * INDEXY))) = Cloud($INDEXX) * Region($INDEXY)"
			echo "- set K8SNODEGROUP in ${CONN_CONFIG[$cspi,$REGION]}"

			echo ""
			echo "[set K8SNODEGROUP autoscale size]"
			test_sequence_change_k8snodegroup_autoscale_size_allcsp $CSP $REGION $POSTFIX $TestSetFile $NUMVM ${0##*/} &
			dozing 3 
		 done
	done
	wait

	# FIXME ../13.k8scluster/status-k8scluster.sh -c $CSP -r $REGION -n $POSTFIX -f $TestSetFile
	#../13.k8scluster/get-k8scluster.sh -c $CSP -r $REGION -n $POSTFIX -f $TestSetFile

else
	echo ""
	echo "[Set K8SNODEGROUP] K8SCLUSTERs(1) = Cloud(1) * Region(1)"

	test_sequence_change_k8snodegroup_autoscale_size $CSP $REGION $POSTFIX $TestSetFile $NUMVM ${0##*/}

fi

duration=$SECONDS

printElapsed $@

echo ""
echo "[Executed Command List]"
cat ./executionStatus
echo ""
