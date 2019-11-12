#!/bin/bash
source ../setup.env


CONNECT_NAME=${CONNECT_NAMES[0]}

#echo ========================== $CONNECT_NAME
#PUBLIC_IPS=`curl -sX GET http://$RESTSERVER:1024/publicip/publicipt01-powerkim?connection_name=azure-northeu-config |json_pp |grep "\"PublicIP\"" |awk '{print $3}' |sed 's/"//g' |sed 's/,//g'`
#for PUBLIC_IP in ${PUBLIC_IPS}
#do
#        echo $CONNECT_NAME : copy testsvc into ${PUBLIC_IP} ...
#	ssh-keygen -f "/root/.ssh/known_hosts" -R ${PUBLIC_IP}
#        scp -i ../keypair/${CONNECT_NAME}.key -o "StrictHostKeyChecking no" ./testsvc/TESTSvc ./testsvc/setup.env cb-user@$PUBLIC_IP:/tmp
#        scp -i ../keypair/${CONNECT_NAME}.key -o "StrictHostKeyChecking no" -r ./testsvc/conf cb-user@$PUBLIC_IP:/tmp
#done

TB_PUBLICIP_IDS=`curl -sX GET http://$TUMBLEBUG_IP:1323/ns/$NS_ID/resources/publicIp | json_pp |grep "\"id\"" |awk '{print $3}' |sed 's/"//g' |sed 's/,//g'`
#echo $TB_PUBLICIP_IDS | json_pp

if [ "$TB_PUBLICIP_IDS" != "" ]
then
	TB_PUBLICIP_IDS=`curl -sX GET http://$TUMBLEBUG_IP:1323/ns/$NS_ID/resources/publicIp |json_pp |grep "\"id\"" |awk '{print $3}' |sed 's/"//g' |sed 's/,//g'`
	for TB_PUBLICIP_ID in ${TB_PUBLICIP_IDS}
	do
		echo ....Get ${TB_PUBLICIP_ID} ...
		PIPS_CONN_NAME=`curl -sX GET http://$TUMBLEBUG_IP:1323/ns/$NS_ID/resources/publicIp/${TB_PUBLICIP_ID} | json_pp | grep "\"connectionName\"" |awk '{print $3}' |sed 's/"//g' |sed 's/,//g'`
		if [ "$PIPS_CONN_NAME" == "$CONNECT_NAME" ]
		then
			PUBLIC_IP=`curl -sX GET http://$TUMBLEBUG_IP:1323/ns/$NS_ID/resources/publicIp/${TB_PUBLICIP_ID} | json_pp | grep "\"publicIp\"" |awk '{print $3}' |sed 's/"//g' |sed 's/,//g'`
			echo $CONNECT_NAME : copy testsvc into ${PUBLIC_IP} ...
			ssh-keygen -f "/root/.ssh/known_hosts" -R ${PUBLIC_IP}
			scp -i ../keypair/${CONNECT_NAME}.key -o "StrictHostKeyChecking no" ./testsvc/TESTSvc ./testsvc/setup.env ubuntu@$PUBLIC_IP:/tmp
			scp -i ../keypair/${CONNECT_NAME}.key -o "StrictHostKeyChecking no" -r ./testsvc/conf ubuntu@$PUBLIC_IP:/tmp
			break
		fi
	done
else
	echo ....no publicIps found
	exit 1
fi
