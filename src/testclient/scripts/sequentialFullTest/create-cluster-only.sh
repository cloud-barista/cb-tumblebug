#!/bin/bash

# Function for individual CSP test
function test_sequence_cluster() {
	local CSP=$1
	local REGION=$2
	local POSTFIX=$3
	local TestSetFile=$4
	local NUMVM=$5 # as DesiredNodeSize
	local CMDPATH=$6

	../13.cluster/create-cluster.sh -c $CSP -r $REGION -n $POSTFIX -f $TestSetFile -x $NUMVM
	dozing 1
	# FIXME ../13.cluster/status-cluster.sh -c $CSP -r $REGION -n $POSTFIX -f $TestSetFile
	../13.cluster/get-cluster.sh -c $CSP -r $REGION -n $POSTFIX -f $TestSetFile

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
	local CMDPATH=$6

	_self=$CMDPATH

	../13.cluster/create-cluster.sh -c $CSP -r $REGION -n $POSTFIX -f $TestSetFile -x $NUMVM
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

if [ "${INDEX}" == "0" ]; then
	echo "[Parallel execution for all CSP regions]"

	INDEXX=${NumCSP}

	for ((cspi = 1; cspi <= INDEXX; cspi++)); do
		INDEXY=${NumRegion[$cspi]}
		CSP=${CSPType[$cspi]}
		for ((cspj = 1; cspj <= INDEXY; cspj++)); do
			#echo $j
			REGION=$cspj

			echo "[Create CLUSTER] CLUSTERs($((INDEXX * INDEXY))) = Cloud($INDEXX) * Region($INDEXY)"
			echo "- Create CLUSTER in ${CONN_CONFIG[$cspi,$cspj]}"

			echo ""
			echo "[Create a Cluster object"
			test_sequence_cluster_allcsp $CSP $REGION $POSTFIX $TestSetFile $NUMVM ${0##*/} &

			# Check CLUSTER object is created
			CLUSTERID=${CLUSTERID_PREFIX}${cspi}${cspj}
			echo ""
			echo "[Waiting for initialization of CLUSTER:$CLUSTERID (5s)]"
			dozing 60

			echo "Checking CLUSTER object. (upto 3s * 20 trials)"
			for ((try = 1; try <= 20; try++)); do
				HTTP_CODE=0
				HTTP_CODE=$(curl -H "${AUTH}" -o /dev/null --write-out "%{http_code}\n" "http://$TumblebugServer/tumblebug/ns/$NSID/cluster/${CLUSTERID}" --silent)
				echo "HTTP status for get CLUSTER object: $HTTP_CODE"
				if [ ${HTTP_CODE} -ge 200 -a ${HTTP_CODE} -le 204 ]; then
					echo "[$try : CLUSTER object is READY]"
					break
				else
					printf "[$try : CLUSTER object is not ready yet].."
					dozing 10
				fi
			done
		 done
	done
	wait

	# FIXME ../13.cluster/status-cluster.sh -c $CSP -r $REGION -n $POSTFIX -f $TestSetFile
	../13.cluster/get-cluster.sh -c $CSP -r $REGION -n $POSTFIX -f $TestSetFile

else
	echo ""
	echo "[Create CLUSTER] CLUSTERs(1) = Cloud(1) * Region(1)"

	test_sequence_cluster $CSP $REGION $POSTFIX $TestSetFile $NUMVM ${0##*/}

fi

duration=$SECONDS

printElapsed $@

echo ""
echo "[Executed Command List]"
cat ./executionStatus
echo ""
