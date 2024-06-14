#!/bin/bash

function clean_sequence_k8scluster() {
	local CSP=$1
	local REGION=$2
	local POSTFIX=$3
	local TestSetFile=$4
	local K8SCLUSTERID_ADD=$5
	local CMDPATH=$6

	../13.k8scluster/delete-k8scluster.sh -c $CSP -r $REGION -n $POSTFIX -f $TestSetFile -x terminate -z $K8SCLUSTERID_ADD
}

function clean_sequence_k8scluster_allcsp() {
	local CSP=$1
	local REGION=$2
	local POSTFIX=$3
	local TestSetFile=$4
	local K8SCLUSTERID_ADD=$5
	local CMDPATH=$6

	_self=$CMDPATH

	../13.k8scluster/delete-k8scluster.sh -c $CSP -r $REGION -n $POSTFIX -f $TestSetFile -x terminate -z $K8SCLUSTERID_ADD
	#dozing 1
	#../8.mcis/status-mcis.sh -c $CSP -r $REGION -n $POSTFIX -f $TestSetFile $MCISPREFIX
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
echo "## Remove K8SCLUSTER only"
echo "####################################################################"

source ../init.sh

K8SCLUSTERID_ADD=${OPTION03:-1}

if [ "${INDEX}" == "0" ]; then
	echo "[Parallel execution for all CSP regions]"

	INDEXX=${NumCSP}
	for ((cspi = 1; cspi <= INDEXX; cspi++)); do
		CSP=${CSPType[$cspi]}
		INDEXY=${NumRegion[$cspi]}

		TOTALK8SCLUSTER=$((INDEXX * INDEXY))
		echo "[DELETE K8SCLUSTER] K8SCLUSTERs($TOTALK8SCLUSTER) in Cloud($CSP) = Cloud($INDEXX) * Region($INDEXY)"

		for ((cspj = 1; cspj <= INDEXY; cspj++)); do
			REGION=$cspj

			echo "- Delete a K8SCLUSTER in ${CONN_CONFIG[$cspi,$cspj]}"

			echo ""
			echo "[Delete a K8SCLUSTER object]"
			clean_sequence_k8scluster_allcsp $CSP $REGION $POSTFIX $TestSetFile $K8SCLUSTERID_ADD ${0##*/} &

			# Check K8SCLUSTER object is deleted
			K8SCLUSTERID=${K8SCLUSTERID_PREFIX}${cspi}${cspj}${K8SCLUSTERID_ADD}
			echo ""
			echo "[Waiting for deleting of K8SCLUSTER:$K8SCLUSTERID (5s)]"
			dozing 5
		 done
	done
	wait

	# FIXME ../13.k8scluster/status-k8scluster.sh -c $CSP -r $REGION -n $POSTFIX -f $TestSetFile
	#../13.k8scluster/get-k8scluster.sh -c $CSP -r $REGION -n $POSTFIX -f $TestSetFile
else
	echo "[Single execution for a CSP region]"
	clean_k8scluster_sequence $CSP $REGION $POSTFIX $TestSetFile $K8SCLUSTERID_ADD ${0##*/}
fi

echo -e "${BOLD}"
echo "[Cleaning related commands in history file executionStatus]"
echo -e ""
echo -e "${NC}${BLUE}- Removing  (K8SCLUSTER) ${CSP} ${REGION} ${POSTFIX} ${TestSetFile}"
echo -e "${NC}"
sed -i "/(K8SCLUSTER) ${CSP} ${REGION} ${POSTFIX} ${TestSetFile//\//\\/}/d" ./executionStatus
echo ""
echo "[Executed Command List]"
cat ./executionStatus
cp ./executionStatus ./executionStatus.back
echo ""

duration=$SECONDS

printElapsed $@
