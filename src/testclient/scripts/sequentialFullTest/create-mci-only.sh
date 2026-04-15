#!/bin/bash

# Function for individual CSP test
function test_sequence() {
	local CSP=$1
	local REGION=$2
	local POSTFIX=$3
	local TestSetFile=$4
	local NUMVM=$5
	local CMDPATH=$6

	../8.infra/create-infra.sh -c $CSP -r $REGION -n $POSTFIX -f $TestSetFile -x $NUMVM 
	dozing 1
	../8.infra/status-infra.sh -c $CSP -r $REGION -n $POSTFIX -f $TestSetFile

	_self=$CMDPATH

	echo ""
	echo "[Logging to notify latest command history]"
	echo "[Infra:${InfraID}(${SECONDS}s)] ${_self} (Infra) ${CSP} ${REGION} ${POSTFIX} ${TestSetFile} ${NUMVM}" >>./executionStatus
	echo ""
	echo "[Executed Command List]"
	#cat ./executionStatus
	cp ./executionStatus ./executionStatus.back
	echo ""
}


function test_sequence_allcsp_infra() {
	local CSP=$1
	local REGION=$2
	local POSTFIX=$3
	local TestSetFile=$4
	local NUMVM=$5

	local CMDPATH=$6

	_self=$CMDPATH

	../8.infra/create-infra.sh -c $CSP -r $REGION -n $POSTFIX -f $TestSetFile -x $NUMVM
	#dozing 1
	#../8.infra/status-infra.sh -c $CSP -r $REGION -n $POSTFIX -f $TestSetFile $InfraPREFIX
	echo ""
	echo "[Logging to notify latest command history]"
	echo "[Infra:${InfraID}(${SECONDS}s+More)] ${_self} (Infra) all 1 ${POSTFIX} ${TestSetFile}" >>./executionStatus
	echo ""
	echo "[Executed Command List]"
	#cat ./executionStatus
	cp ./executionStatus ./executionStatus.back
	echo ""

}

function test_sequence_allcsp_infra_vm() {
	local CSP=$1
	local REGION=$2
	local POSTFIX=$3
	local TestSetFile=$4
	local NUMVM=$5

	../8.infra/add-nodegroup-to-infra.sh -c $CSP -r $REGION -n $POSTFIX -f $TestSetFile -x $NUMVM 

}
SECONDS=0

echo "####################################################################"
echo "## Create resource-ns-cloud from Zero Base"
echo "####################################################################"

source ../init.sh

NUMVM=${OPTION01:-1}


if [ "${INDEX}" == "0" ]; then
	echo "[Parallel execution for all CSP regions]"

	INDEXX=${NumCSP}


	InfraID=${POSTFIX}


	for ((cspi = 1; cspi <= INDEXX; cspi++)); do
		#echo $i
		INDEXY=${NumRegion[$cspi]}
		CSP=${CSPType[$cspi]}
		for ((cspj = 1; cspj <= INDEXY; cspj++)); do
			#echo $j
			REGION=$cspj

			#echo $CSP
			#echo $REGION
			TOTALVM=$((INDEXX * INDEXY * NUMVM))
			echo "[Create Infra] VMs($TOTALVM) = Cloud($INDEXX) * Region($INDEXY) * nodeGroup($NUMVM)"
			echo "- Create VM in ${CONN_CONFIG[$cspi, $REGION]}"

			if [ "${cspi}" -eq 1 ] && [ "${cspj}" -eq 1 ]; then
				echo ""
				echo "[Create a Infra object with a VM]"
				test_sequence_allcsp_infra $CSP $REGION $POSTFIX $TestSetFile $NUMVM ${0##*/} &
				# Check Infra object is created
				echo ""
				echo "[Waiting for initialization of Infra:$InfraID (5s)]"
				dozing 5

				echo "Checking Infra object. (upto 3s * 20 trials)"
				for ((try = 1; try <= 20; try++)); do
					HTTP_CODE=0
					HTTP_CODE=$(curl -H "${AUTH}" -o /dev/null --write-out "%{http_code}\n" "http://$TumblebugServer/tumblebug/ns/$NSID/infra/${InfraID}" --silent)
					echo "HTTP status for get Infra object: $HTTP_CODE"
					if [ ${HTTP_CODE} -ge 200 -a ${HTTP_CODE} -le 204 ]; then
						echo "[$try : Infra object is READY]"
						break
					else
						printf "[$try : Infra object is not ready yet].."
						dozing 3
					fi
				done
			else
				dozing 6
				echo ""
				echo "[Create VM and add it into the Infra in parallel]"
				test_sequence_allcsp_infra_vm $CSP $REGION $POSTFIX $TestSetFile $NUMVM &

			fi
		done

	done
	wait

	../8.infra/status-infra.sh -c $CSP -r $REGION -n $POSTFIX -f $TestSetFile

else
	echo ""
	TOTALVM=$((1 * 1 * NUMVM))
	echo "[Create Infra] VMs($TOTALVM) = Cloud(1) * Region(1) * nodeGroup($NUMVM)"

	InfraID=${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX}

	test_sequence $CSP $REGION $POSTFIX $TestSetFile $NUMVM ${0##*/}

fi

duration=$SECONDS

printElapsed $@

echo ""
echo "[Executed Command List]"
cat ./executionStatus
echo ""
