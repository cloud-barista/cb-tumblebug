#!/bin/bash

#function create_mcis_policy() {


	TestSetFile=${5:-../testSet.env}
    
    if [ ! -f "$TestSetFile" ]; then
        echo "$TestSetFile does not exist."
        exit
    fi
	source $TestSetFile
    source ../conf.env
	
	echo "####################################################################"
	echo "## 8. Create MCIS Policy"
	echo "####################################################################"

	CSP=${1}
	REGION=${2:-1}
	POSTFIX=${3:-developer}
	MCISNAME=${4:-noname}

	source ../common-functions.sh
	getCloudIndex $CSP


	MCISID=${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX}

	if [ "${INDEX}" == "0" ]; then
		# MCISPREFIX=avengers
		MCISID=${MCISPREFIX}-${POSTFIX}
	fi

	if [ "${MCISNAME}" != "noname" ]; then
		echo "[MCIS name is given]"
		MCISID=${MCISNAME}
	fi

	$CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/cbadm mcis create-policy --config $CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/grpc_conf.yaml  -i json -o json -d \
	"{
		\"nsId\":  \"${NSID}\",
		\"mcisId\":  \"${MCISID}\",
		\"ReqInfo\": {
			\"description\": \"Tumblebug Auto Control Demo\",
			\"policy\": [
				{
					\"autoCondition\": {
						\"metric\": \"cpu\",
						\"operator\": \">=\",
						\"operand\": \"80\",
						\"evaluationPeriod\": \"10\"
					},
					\"autoAction\": {
						\"actionType\": \"ScaleOut\",
						\"placementAlgo\": \"random\",
						\"vm\": {
							\"name\": \"AutoGen\"
						},
						\"postCommand\": {
							\"command\": \"wget https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/scripts/setweb.sh -O ~/setweb.sh; chmod +x ~/setweb.sh; sudo ~/setweb.sh; wget https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/scripts/runLoadMaker.sh -O ~/runLoadMaker.sh; chmod +x ~/runLoadMaker.sh; sudo ~/runLoadMaker.sh\"
						}
					}
				},				
				{
					\"autoCondition\": {
						\"metric\": \"cpu\",
						\"operator\": \"<=\",
						\"operand\": \"60\",
						\"evaluationPeriod\": \"10\"
					},
					\"autoAction\": {
						\"actionType\": \"ScaleIn\"
					}
				}
			]
		}
	}" | jq '' 
#}

#create_mcis