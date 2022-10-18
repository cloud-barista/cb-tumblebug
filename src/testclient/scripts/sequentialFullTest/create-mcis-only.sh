#!/bin/bash

# Function for individual CSP test
function test_sequence() {
	local CSP=$1
	local REGION=$2
	local POSTFIX=$3
	local TestSetFile=$4
	local NUMVM=$5
	local CMDPATH=$6

	../8.mcis/create-mcis.sh -c $CSP -r $REGION -n $POSTFIX -f $TestSetFile -x $NUMVM 
	dozing 1
	../8.mcis/status-mcis.sh -c $CSP -r $REGION -n $POSTFIX -f $TestSetFile

	_self=$CMDPATH

	echo ""
	echo "[Logging to notify latest command history]"
	echo "[MCIS:${MCISID}(${SECONDS}s)] ${_self} (MCIS) ${CSP} ${REGION} ${POSTFIX} ${TestSetFile} ${NUMVM}" >>./executionStatus
	echo ""
	echo "[Executed Command List]"
	#cat ./executionStatus
	cp ./executionStatus ./executionStatus.back
	echo ""
}


function test_sequence_allcsp_mcis() {
	local CSP=$1
	local REGION=$2
	local POSTFIX=$3
	local TestSetFile=$4
	local NUMVM=$5

	local CMDPATH=$6

	_self=$CMDPATH

	../8.mcis/create-mcis.sh -c $CSP -r $REGION -n $POSTFIX -f $TestSetFile -x $NUMVM
	#dozing 1
	#../8.mcis/status-mcis.sh -c $CSP -r $REGION -n $POSTFIX -f $TestSetFile $MCISPREFIX
	echo ""
	echo "[Logging to notify latest command history]"
	echo "[MCIS:${MCISID}(${SECONDS}s+More)] ${_self} (MCIS) all 1 ${POSTFIX} ${TestSetFile}" >>./executionStatus
	echo ""
	echo "[Executed Command List]"
	#cat ./executionStatus
	cp ./executionStatus ./executionStatus.back
	echo ""

}

function test_sequence_allcsp_mcis_vm() {
	local CSP=$1
	local REGION=$2
	local POSTFIX=$3
	local TestSetFile=$4
	local NUMVM=$5

	../8.mcis/add-subgroup-to-mcis.sh -c $CSP -r $REGION -n $POSTFIX -f $TestSetFile -x $NUMVM 

}
SECONDS=0

echo "####################################################################"
echo "## Create mcir-ns-cloud from Zero Base"
echo "####################################################################"

source ../init.sh

NUMVM=${OPTION01:-1}


if [ "${INDEX}" == "0" ]; then
	echo "[Parallel execution for all CSP regions]"

	INDEXX=${NumCSP}


	MCISID=${POSTFIX}


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
			echo "[Create MCIS] VMs($TOTALVM) = Cloud($INDEXX) * Region($INDEXY) * subGroup($NUMVM)"
			echo "- Create VM in ${CONN_CONFIG[$cspi, $REGION]}"

			if [ "${cspi}" -eq 1 ] && [ "${cspj}" -eq 1 ]; then
				echo ""
				echo "[Create a MCIS object with a VM]"
				test_sequence_allcsp_mcis $CSP $REGION $POSTFIX $TestSetFile $NUMVM ${0##*/} &
				# Check MCIS object is created
				echo ""
				echo "[Waiting for initialization of MCIS:$MCISID (5s)]"
				dozing 5

				echo "Checking MCIS object. (upto 3s * 20 trials)"
				for ((try = 1; try <= 20; try++)); do
					HTTP_CODE=0
					HTTP_CODE=$(curl -H "${AUTH}" -o /dev/null --write-out "%{http_code}\n" "http://$TumblebugServer/tumblebug/ns/$NSID/mcis/${MCISID}" --silent)
					echo "HTTP status for get MCIS object: $HTTP_CODE"
					if [ ${HTTP_CODE} -ge 200 -a ${HTTP_CODE} -le 204 ]; then
						echo "[$try : MCIS object is READY]"
						break
					else
						printf "[$try : MCIS object is not ready yet].."
						dozing 3
					fi
				done
			else
				dozing 6
				echo ""
				echo "[Create VM and add it into the MCIS in parallel]"
				test_sequence_allcsp_mcis_vm $CSP $REGION $POSTFIX $TestSetFile $NUMVM &

			fi
		done

	done
	wait

	../8.mcis/status-mcis.sh -c $CSP -r $REGION -n $POSTFIX -f $TestSetFile

else
	echo ""
	TOTALVM=$((1 * 1 * NUMVM))
	echo "[Create MCIS] VMs($TOTALVM) = Cloud(1) * Region(1) * subGroup($NUMVM)"

	MCISID=${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX}

	test_sequence $CSP $REGION $POSTFIX $TestSetFile $NUMVM ${0##*/}

fi

duration=$SECONDS

printElapsed $@

echo ""
echo "[Executed Command List]"
cat ./executionStatus
echo ""
