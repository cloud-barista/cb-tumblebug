#!/bin/bash

# Function for individual CSP test
function test_sequence_set_nodegroup_autoscaling() {
	local CSP=$1
	local REGION=$2
	local POSTFIX=$3
	local TestSetFile=$4
	local NUMVM=$5 # as DesiredNodeSize
	local CMDPATH=$6

	../13.cluster/set-autoscaling.sh -c $CSP -r $REGION -n $POSTFIX -f $TestSetFile -x $NUMVM 
	dozing 1
	# FIXME ../13.cluster/status-cluster.sh -c $CSP -r $REGION -n $POSTFIX -f $TestSetFile
	#../13.cluster/get-cluster.sh -c $CSP -r $REGION -n $POSTFIX -f $TestSetFile

	_self=$CMDPATH

	CLUSTERID=${CLUSTERID_PREFIX}${cspi}${cspj}
	echo ""
	echo "[Logging to notify latest command history]"
	echo "[CLUSTER:${CLUSTERID}(${SECONDS}s)] ${_self} (Cluster) ${CSP} ${REGION} ${POSTFIX} ${TestSetFile} ${DesiredNodeSize} ${MinNodeSize} ${MaxNodeSize}" >>./executionStatus
	echo ""
	echo "[Executed Command List]"
	#cat ./executionStatus
	cp ./executionStatus ./executionStatus.back
	echo ""
}


function test_sequence_set_nodegroup_autoscaling_allcsp() {
	local CSP=$1
	local REGION=$2
	local POSTFIX=$3
	local TestSetFile=$4
	local NUMVM=$5 # as DesiredNodeSize
	local CMDPATH=$6

	_self=$CMDPATH

	../13.cluster/set-autoscaling.sh -c $CSP -r $REGION -n $POSTFIX -f $TestSetFile -x $NUMVM
	#dozing 1
	#../8.mcis/status-mcis.sh -c $CSP -r $REGION -n $POSTFIX -f $TestSetFile $MCISPREFIX
	#../13.cluster/get-cluster.sh -c $CSP -r $REGION -n $POSTFIX -f $TestSetFile

	echo ""
	echo "[Logging to notify latest command history]"
	echo "[CLUSTER:${CLUSTERID}(${SECONDS}s+More)] ${_self} (CLUSTER) all 1 ${POSTFIX} ${TestSetFile}" >>./executionStatus
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

			echo "[set NODEGROUP] CLUSTERs($((INDEXX * INDEXY))) = Cloud($INDEXX) * Region($INDEXY)"
			echo "- set NODEGROUP in ${CONN_CONFIG[$cspi,$REGION]}"

			echo ""
			echo "[set NODEGROUP autoscaling]"
			test_sequence_set_nodegroup_autoscaling_allcsp $CSP $REGION $POSTFIX $TestSetFile $NUMVM ${0##*/} &
			dozing 3 
		 done
	done
	wait

	# FIXME ../13.cluster/status-cluster.sh -c $CSP -r $REGION -n $POSTFIX -f $TestSetFile
	#../13.cluster/get-cluster.sh -c $CSP -r $REGION -n $POSTFIX -f $TestSetFile

else
	echo ""
	echo "[Set NODEGROUP] CLUSTERs(1) = Cloud(1) * Region(1)"

	test_sequence_set_nodegroup_autoscaling $CSP $REGION $POSTFIX $TestSetFile $NUMVM ${0##*/}

fi

duration=$SECONDS

printElapsed $@

echo ""
echo "[Executed Command List]"
cat ./executionStatus
echo ""
