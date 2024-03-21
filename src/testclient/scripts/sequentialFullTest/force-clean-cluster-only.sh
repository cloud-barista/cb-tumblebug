#!/bin/bash

function clean_sequence_cluster() {
	local CSP=$1
	local REGION=$2
	local POSTFIX=$3
	local TestSetFile=$4
	local CLUSTERID_ADD=$5
	local CMDPATH=$6

	../13.cluster/force-delete-cluster.sh -c $CSP -r $REGION -n $POSTFIX -f $TestSetFile -x terminate -z $CLUSTERID_ADD
}

function clean_sequence_cluster_allcsp() {
	local CSP=$1
	local REGION=$2
	local POSTFIX=$3
	local TestSetFile=$4
	local CLUSTERID_ADD=$5
	local CMDPATH=$6

	_self=$CMDPATH

	../13.cluster/force-delete-cluster.sh -c $CSP -r $REGION -n $POSTFIX -f $TestSetFile -x terminate -z $CLUSTERID_ADD
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
echo "## Remove CLUSTER only"
echo "####################################################################"

source ../init.sh

CLUSTERID_ADD=${OPTION03:-1}

if [ "${INDEX}" == "0" ]; then
	echo "[Parallel execution for all CSP regions]"

	INDEXX=${NumCSP}
	for ((cspi = 1; cspi <= INDEXX; cspi++)); do
		CSP=${CSPType[$cspi]}
		INDEXY=${NumRegion[$cspi]}

		TOTALCLUSTER=$((INDEXX * INDEXY))
		echo "[DELETE CLUSTER] CLUSTERs($TOTALCLUSTER) in Cloud($CSP) = Cloud($INDEXX) * Region($INDEXY)"

		for ((cspj = 1; cspj <= INDEXY; cspj++)); do
			REGION=$cspj

			echo "- Delete a CLUSTER in ${CONN_CONFIG[$cspi,$cspj]}"

			echo ""
			echo "[Delete a CLUSTER object]"
			clean_sequence_cluster_allcsp $CSP $REGION $POSTFIX $TestSetFile $CLUSTERID_ADD ${0##*/} &

			# Check CLUSTER object is deleted
			CLUSTERID=${CLUSTERID_PREFIX}${cspi}${cspj}${CLUSTERID_ADD}
			echo ""
			echo "[Waiting for deleting of CLUSTER:$CLUSTERID (5s)]"
			dozing 5

<<COMMENT
			echo "Checking a CLUSTER object. (upto 5s * 10 trials)"
			for ((try = 1; try <= 10; try++)); do
				HTTP_CODE=0
				HTTP_CODE=$(curl -H "${AUTH}" -o /dev/null --write-out "%{http_code}\n" "http://$TumblebugServer/tumblebug/ns/$NSID/cluster/${CLUSTERID}" --silent)
				echo "HTTP status for get CLUSTER object: $HTTP_CODE"
				if [ ${HTTP_CODE} -ge 200 -a ${HTTP_CODE} -le 204 ]; then
					echo "[$try : CLUSTER object is still ALIVE].."
					dozing 5
				else
					printf "[$try : CLUSTER object is deleted or not existed].."
					break
				fi
			done
COMMENT			
		 done
	done
	wait

	# FIXME ../13.cluster/status-cluster.sh -c $CSP -r $REGION -n $POSTFIX -f $TestSetFile
	#../13.cluster/get-cluster.sh -c $CSP -r $REGION -n $POSTFIX -f $TestSetFile
else
	echo "[Single execution for a CSP region]"
	clean_cluster_sequence $CSP $REGION $POSTFIX $TestSetFile $CLUSTERID_ADD ${0##*/}
fi

echo -e "${BOLD}"
echo "[Cleaning related commands in history file executionStatus]"
echo -e ""
echo -e "${NC}${BLUE}- Removing  (CLUSTER) ${CSP} ${REGION} ${POSTFIX} ${TestSetFile}"
echo -e "${NC}"
sed -i "/(CLUSTER) ${CSP} ${REGION} ${POSTFIX} ${TestSetFile//\//\\/}/d" ./executionStatus
echo ""
echo "[Executed Command List]"
cat ./executionStatus
cp ./executionStatus ./executionStatus.back
echo ""

duration=$SECONDS

printElapsed $@
