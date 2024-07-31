#!/bin/bash


echo "####################################################################"
echo "## Deploy CB-Dragonfly"
echo "####################################################################"

SECONDS=0

source ../init.sh

if [ "${INDEX}" == "0" ]; then
	# MCISPREFIX=avengers
	MCISID=${POSTFIX}
fi

CMD="wget https://raw.githubusercontent.com/cloud-barista/cb-tumblebug/main/scripts/setcbdf.sh -O ~/setcbdf.sh; chmod +x ~/setcbdf.sh; ~/setcbdf.sh"
echo "CMD: $CMD"

	curl -H "${AUTH}" -sX POST http://$TumblebugServer/tumblebug/ns/$NSID/cmd/mcis/$MCISID -H 'Content-Type: application/json' -d \
		'{
			"command"        : "[${CMD}]"
		}' | jq '' #|| return 1

	MCISINFO=`curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NSID/mcis/${MCISID}?option=status`
	MASTERIP=$(jq -r '.status.masterIp' <<< "$MCISINFO")
	MASTERVM=$(jq -r '.status.masterVmId' <<< "$MCISINFO")
	
	echo "MASTERIP: $MASTERIP"
	echo "MASTERVM: $MASTERVM"

	# Check CB-Dragonfly is healthy
	echo ""
	echo "[Waiting for initialization of CB-Dragonfly. (20s)]"
	dozing 20

	echo "Checking CB-Dragonfly Status. (upto 5s * 20 trials)" 

	for (( try=1; try<=20; try++ ))
	do
		HTTP_CODE=$(curl --write-out "%{http_code}\n" "http://${MASTERIP}:9090/dragonfly/healthcheck" --silent)
		echo "CB-Dragonfly Status: $HTTP_CODE" 
		if [ ${HTTP_CODE} -ge 200 -a ${HTTP_CODE} -le 204 ]; then
			break
		else 
			printf "[$try : NOT Healthy.].."
			dozing 5
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
	PARAM="-x TB_DRAGONFLY_REST_URL -y http://${MASTERIP}:9090/dragonfly"
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