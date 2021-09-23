#!/bin/bash

#function deploy_nginx_to_mcis() {


	echo "[Check jq package (if not, install)]"
	if ! dpkg-query -W -f='${Status}' jq  | grep "ok installed"; then sudo apt install -y jq; fi
	

	TestSetFile=${4:-../testSet.env}
    if [ ! -f "$TestSetFile" ]; then
        echo "$TestSetFile does not exist."
        exit
    fi
	source $TestSetFile
    source ../conf.env
	
	echo "####################################################################"
	echo "## Command (SSH) to MCIS "
	echo "####################################################################"

	CSP=${1}
	REGION=${2:-1}
	POSTFIX=${3:-developer}

	source ../common-functions.sh
	getCloudIndex $CSP


	MCISID=${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX}

	if [ "${INDEX}" == "0" ]; then
		# MCISPREFIX=avengers
		MCISID=${MCISPREFIX}-${POSTFIX}
	fi

	$CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/cbadm mcis command --config $CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/grpc_conf.yaml -i json -o json -d \
	"{
		\"nsId\":  \"${NSID}\",
		\"mcisId\": \"${MCISID}\",
		\"cmd\": {
			\"command\": \"wget https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/scripts/setcbdf.sh -O ~/setcbdf.sh; chmod +x ~/setcbdf.sh; ~/setcbdf.sh\"
		}
	}" | jq '' #|| return 1

	MCISINFO=`$CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/cbadm mcis status --config $CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/grpc_conf.yaml -o json --ns $NSID --mcis ${MCISID}`
	MASTERIP=$(jq -r '.status.masterIp' <<< "$MCISINFO")
	MASTERVM=$(jq -r '.status.masterVmId' <<< "$MCISINFO")
	
	echo "MASTERIP: $MASTERIP"
	echo "MASTERVM: $MASTERVM"

	# Check CB-Dragonfly is healthy
	echo ""
	echo "[Waiting for initialization of CB-Dragonfly. (20s)]"
	dozing 20

	echo "Checking CB-Dragonfly Status. (upto 3s * 20 trials)" 

	for (( try=1; try<=20; try++ ))
	do
		HTTP_CODE=$(curl --write-out "%{http_code}\n" "http://${MASTERIP}:9090/dragonfly/healthcheck" --silent)
		echo "CB-Dragonfly Status: $HTTP_CODE" 
		if [ ${HTTP_CODE} -ge 200 -a ${HTTP_CODE} -le 204 ]; then
			break
		else 
			printf "[$try : NOT Healthy.].."
			dozing 3
		fi
	done

	HTTP_CODE=$(curl --write-out "%{http_code}\n" "http://${MASTERIP}:9090/dragonfly/healthcheck" --silent)
	echo "CB-Dragonfly Status: $HTTP_CODE" 
	if [ ${HTTP_CODE} -ge 200 -a ${HTTP_CODE} -le 204 ]; then
		echo "CB-Dragonfly is healthy..!"
	else 
		echo "CB-Dragonfly is NOT healthy. We need to check manually. (Note: CB-DF needs 2 vCPU and 4GiB Memory at least)"
	fi


	echo "If CB-Dragonfly is not responding, We need to wait more."
	echo ""

	echo "[Update Tumblebug Environment for Dragonfly with following command]"
	PARAM="DRAGONFLY_REST_URL http://${MASTERIP}:9090/dragonfly"
	echo $PARAM
	../2.configureTumblebug/update-config.sh $PARAM

	echo ""
	echo "[You can test Dragonfly with following command]"
	echo " ../9.monitoring/install-agent.sh ${CSP} ${REGION} ${POSTFIX}"
	echo " ../9.monitoring/get-monitoring-data.sh ${CSP} ${REGION} ${POSTFIX} cpu"

	echo ""
	echo "[You can check Dragonfly monitoring status with Chronograf]"
	echo " (In Chronograf, you need to create a Dashboard view by selecting metrics)"
	echo " http://${MASTERIP}:8888/sources/0/dashboards"

#}

#deploy_cb-df_to_mcis