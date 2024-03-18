#!/bin/bash

# Function for individual CSP test
function test_sequence_cluster() {
	local CSP=$1
	local REGION=$2
	local POSTFIX=$3
	local TestSetFile=$4
	local NUMVM=$5 # as DesiredNodeSize
	local CLUSTERID_ADD=$6
	local CMDPATH=$7

	../13.cluster/get-cluster.sh -c $CSP -r $REGION -n $POSTFIX -f $TestSetFile -z $CLUSTERID_ADD

	_self=$CMDPATH

	echo ""
	echo "[Logging to notify latest command history]"
	echo "[CLUSTER:${CLUSTERID}(${SECONDS}s)] ${_self} (Cluster) ${CSP} ${REGION} ${POSTFIX} ${TestSetFile} ${DesiredNodeSize} ${MinNodeSize} ${MaxNodeSize}" >>./executionStatus
	echo ""
	echo "[Executed Command List]"
	#cat ./executionStatus
	cp ./executionStatus ./executionStatus.back
	echo ""
}


function test_sequence_cluster_allcsp() {
	local CSP=$1
	local REGION=$2
	local POSTFIX=$3
	local TestSetFile=$4
	local NUMVM=$5 # as DesiredNodeSize
	local CLUSTERID_ADD=$6
	local CMDPATH=$7

	_self=$CMDPATH

	../13.cluster/get-cluster.sh -c $CSP -r $REGION -n $POSTFIX -f $TestSetFile -z $CLUSTERID_ADD
	#dozing 1
	#../8.mcis/status-mcis.sh -c $CSP -r $REGION -n $POSTFIX -f $TestSetFile $MCISPREFIX
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
echo "## Create cluster from Zero Base"
echo "####################################################################"

source ../init.sh

NUMVM=${OPTION01:-1}
CLUSTERID_ADD=${OPTION03:-1}

if [ "${INDEX}" == "0" ]; then
	echo "[Parallel execution for all CSP regions]"

	INDEXX=${NumCSP}

	for ((cspi = 1; cspi <= INDEXX; cspi++)); do
		INDEXY=${NumRegion[$cspi]}
		CSP=${CSPType[$cspi]}
		for ((cspj = 1; cspj <= INDEXY; cspj++)); do
			#echo $j
			REGION=$cspj

			echo "[Get CLUSTER] CLUSTERs($((INDEXX * INDEXY))) = Cloud($INDEXX) * Region($INDEXY)"
			echo "- Get CLUSTER in ${CONN_CONFIG[$cspi,$cspj]}"

			echo ""
			echo "[Get a Cluster object]"
			test_sequence_cluster_allcsp $CSP $REGION $POSTFIX $TestSetFile $NUMVM $CLUSTERID_ADD ${0##*/} &
			dozing 3 
		 done
	done
	wait
else
	echo ""
	echo "[Create CLUSTER] CLUSTERs(1) = Cloud(1) * Region(1)"

	test_sequence_cluster $CSP $REGION $POSTFIX $TestSetFile $NUMVM $CLUSTERID_ADD ${0##*/}

fi

duration=$SECONDS

printElapsed $@

echo ""
echo "[Executed Command List]"
cat ./executionStatus
echo ""
