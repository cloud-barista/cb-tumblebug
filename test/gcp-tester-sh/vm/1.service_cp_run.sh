#!/bin/bash
source ../setup.env


CONNECT_NAME=${CONNECT_NAMES[0]}

#num=0

#echo ========================== $CONNECT_NAME
#PUBLIC_IPS=`curl -sX GET http://$RESTSERVER:1024/publicip/publicipt${num}-powerkim?connection_name=$CONNECT_NAME |json_pp |grep "\"PublicIP\"" |awk '{print $3}' |sed 's/"//g' |sed 's/,//g'`
## 137.135.167.9
#for PUBLIC_IP in ${PUBLIC_IPS}
#do
#        echo $CONNECT_NAME : copy testsvc into ${PUBLIC_IP} ...
#	ssh-keygen -f "/root/.ssh/known_hosts" -R ${PUBLIC_IP}
#        scp -i ../keypair/${CONNECT_NAME}.key -o "StrictHostKeyChecking no" ./testsvc/TESTSvc ./testsvc/setup.env cb-user@$PUBLIC_IP:/tmp
#        scp -i ../keypair/${CONNECT_NAME}.key -o "StrictHostKeyChecking no" -r ./testsvc/conf cb-user@$PUBLIC_IP:/tmp
#done

TB_PUBLICIP_IDS=`curl -sX GET http://$TUMBLEBUG_IP:1323/ns/$NS_ID/resources/publicIp | jq -r '.publicIp[].id'`
#echo $TB_PUBLICIP_IDS | json_pp

if [ -n "$TB_PUBLICIP_IDS" ]
then
        #TB_PUBLICIP_IDS=`curl -sX GET http://$TUMBLEBUG_IP:1323/ns/$NS_ID/resources/publicIp | jq -r '.publicIp[].id'`
        for TB_PUBLICIP_ID in ${TB_PUBLICIP_IDS}
        do
                echo ....Get ${TB_PUBLICIP_ID} ...
                PIPS_CONN_NAME=`curl -sX GET http://$TUMBLEBUG_IP:1323/ns/$NS_ID/resources/publicIp/${TB_PUBLICIP_ID} | jq -r '.connectionName'`
                if [ "$PIPS_CONN_NAME" == "$CONNECT_NAME" ]
                then
                        PUBLIC_IP=`curl -sX GET http://$TUMBLEBUG_IP:1323/ns/$NS_ID/resources/publicIp/${TB_PUBLICIP_ID} | jq -r '.publicIp'`
                        echo $CONNECT_NAME : copy testsvc into ${PUBLIC_IP} ...
                        ssh-keygen -f "/root/.ssh/known_hosts" -R ${PUBLIC_IP}
                        scp -i ../keypair/${CONNECT_NAME}.key -o "StrictHostKeyChecking no" ./testsvc/TESTSvc ./testsvc/setup.env cb-user@$PUBLIC_IP:/tmp
                        scp -i ../keypair/${CONNECT_NAME}.key -o "StrictHostKeyChecking no" -r ./testsvc/conf cb-user@$PUBLIC_IP:/tmp
                        break
                fi
        done
else
        echo ....no publicIps found
        exit 1
fi
