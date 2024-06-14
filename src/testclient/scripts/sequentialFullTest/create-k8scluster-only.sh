#!/bin/bash

# Function for individual CSP test
function test_sequence_k8scluster() {
	local CSP=$1
	local REGION=$2
	local POSTFIX=$3
	local TestSetFile=$4
	local NUMVM=$5 # as DesiredNodeSize
	local K8SCLUSTERID_ADD=$6
	local CMDPATH=$7

	../13.k8scluster/create-k8scluster.sh -c $CSP -r $REGION -n $POSTFIX -f $TestSetFile -x $NUMVM -z $K8SCLUSTERID_ADD
	dozing 1
	# FIXME ../13.k8scluster/status-k8scluster.sh -c $CSP -r $REGION -n $POSTFIX -f $TestSetFile
	#../13.k8scluster/get-k8scluster.sh -c $CSP -r $REGION -n $POSTFIX -f $TestSetFile -z $K8SCLUSTERID_ADD

	_self=$CMDPATH

	#CNAME=${K8SCLUSTERID}${cspi}${cspj}
	echo ""
	echo "[Logging to notify latest command history]"
	echo "[K8SCLUSTER:${K8SCLUSTERID}(${SECONDS}s)] ${_self} (K8sCluster) ${CSP} ${REGION} ${POSTFIX} ${TestSetFile} ${DesiredNodeSize} ${MinNodeSize} ${MaxNodeSize}" >>./executionStatus
	echo ""
	echo "[Executed Command List]"
	#cat ./executionStatus
	cp ./executionStatus ./executionStatus.back
	echo ""
}


function test_sequence_k8scluster_allcsp() {
	local CSP=$1
	local REGION=$2
	local POSTFIX=$3
	local TestSetFile=$4
	local NUMVM=$5 # as DesiredNodeSize
	local K8SCLUSTERID_ADD=$6
	local CMDPATH=$7

	_self=$CMDPATH

	../13.k8scluster/create-k8scluster.sh -c $CSP -r $REGION -n $POSTFIX -f $TestSetFile -x $NUMVM -z $K8SCLUSTERID_ADD
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
echo "## Create k8scluster from Zero Base"
echo "####################################################################"

source ../init.sh

NUMVM=${OPTION01:-1}
K8SCLUSTERID_ADD=${OPTION03:-1}

if [ "${INDEX}" == "0" ]; then
	echo "[Parallel execution for all CSP regions]"

	INDEXX=${NumCSP}

	for ((cspi = 1; cspi <= INDEXX; cspi++)); do
		INDEXY=${NumRegion[$cspi]}
		CSP=${CSPType[$cspi]}
		for ((cspj = 1; cspj <= INDEXY; cspj++)); do
			#echo $j
			REGION=$cspj

			echo "[Create K8SCLUSTER] K8SCLUSTERs($((INDEXX * INDEXY))) = Cloud($INDEXX) * Region($INDEXY)"
			echo "- Create K8SCLUSTER in ${CONN_CONFIG[$cspi,$cspj]}"

			echo ""
			echo "[Create a K8sCluster object"
			test_sequence_k8scluster_allcsp $CSP $REGION $POSTFIX $TestSetFile $NUMVM $K8SCLUSTERID_ADD ${0##*/} &

			# Check K8SCLUSTER object is created
			K8SCLUSTERID=${K8SCLUSTERID_PREFIX}${cspi}${cspj}${K8SCLUSTERID_ADD}
			echo ""
			echo "[Waiting for initialization of K8SCLUSTER:$CNAME (3s)]"
			dozing 3 

<<COMMENT
			echo "Checking K8SCLUSTER object. (upto 3s * 20 trials)"
			for ((try = 1; try <= 20; try++)); do
				HTTP_CODE=0
				HTTP_CODE=$(curl -H "${AUTH}" -o /dev/null --write-out "%{http_code}\n" "http://$TumblebugServer/tumblebug/ns/$NSID/k8scluster/${K8SCLUSTERID}" --silent)
				echo "HTTP status for get K8SCLUSTER object: $HTTP_CODE"
				if [ ${HTTP_CODE} -ge 200 -a ${HTTP_CODE} -le 204 ]; then
					echo "[$try : K8SCLUSTER object is READY]"
					break
				else
					printf "[$try : K8SCLUSTER object is not ready yet].."
					dozing 10
				fi
			done
COMMENT
		 done
	done
	wait

	# FIXME ../13.k8scluster/status-k8scluster.sh -c $CSP -r $REGION -n $POSTFIX -f $TestSetFile
	#../13.k8scluster/get-k8scluster.sh -c $CSP -r $REGION -n $POSTFIX -f $TestSetFile -z $K8SCLUSTERID_ADD

else
	echo ""
	echo "[Create K8SCLUSTER] K8SCLUSTERs(1) = Cloud(1) * Region(1)"

	test_sequence_k8scluster $CSP $REGION $POSTFIX $TestSetFile $NUMVM $K8SCLUSTERID_ADD ${0##*/}
fi

duration=$SECONDS

printElapsed $@

echo ""
echo "[Executed Command List]"
cat ./executionStatus
echo ""
