#!/bin/bash
source ../setup.env

#KEY_NAME=${CONNECT_NAMES[0]}

#num=0
#for NAME in "${CONNECT_NAMES[@]}"
#do
#	if [ $num -eq 0 ] ; then
#		echo .... first vm skipped!!
#	else
#		echo ========================== $NAME
#		PUBLIC_IPS=`curl -sX GET http://$RESTSERVER:1024/vm?connection_name=$NAME |json_pp |grep "\"PublicIP\"" |awk '{print $3}' |sed 's/"//g' |sed 's/,//g'`
#		for PUBLIC_IP in ${PUBLIC_IPS}
#		do
#			echo $NAME: copy shooter into ${PUBLIC_IP} ...
#			ssh-keygen -f "/root/.ssh/known_hosts" -R ${PUBLIC_IP}
#			scp -i ../keypair/$KEY_NAME.key -o "StrictHostKeyChecking no" ./shooter/shooter.sh cb-user@$PUBLIC_IP:/tmp
#			ssh -i ../keypair/$KEY_NAME.key -o "StrictHostKeyChecking no" cb-user@$PUBLIC_IP /tmp/shooter.sh &
#		done
#		
#	fi
#	num=`expr $num + 1`
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
                if [ "$PIPS_CONN_NAME" == "${CONNECT_NAMES[0]}" ]
                then
                        echo Skipping first VM
                        continue
                else
                        PUBLIC_IP=`curl -sX GET http://$TUMBLEBUG_IP:1323/ns/$NS_ID/resources/publicIp/${TB_PUBLICIP_ID} | jq -r '.publicIp'`
                        echo $PIPS_CONN_NAME: copy shooter into ${PUBLIC_IP} ...
                        ssh-keygen -f "/root/.ssh/known_hosts" -R ${PUBLIC_IP}
                        scp -i ../keypair/$PIPS_CONN_NAME.key -o "StrictHostKeyChecking no" ./shooter/shooter.sh ubuntu@$PUBLIC_IP:/tmp
                        ssh -i ../keypair/$PIPS_CONN_NAME.key -o "StrictHostKeyChecking no" ubuntu@$PUBLIC_IP /tmp/shooter.sh & > /dev/null
                        scp -i ../keypair/$PIPS_CONN_NAME.key -o "StrictHostKeyChecking no" ./shooter/shooter.sh cb-user@$PUBLIC_IP:/tmp
                        ssh -i ../keypair/$PIPS_CONN_NAME.key -o "StrictHostKeyChecking no" cb-user@$PUBLIC_IP /tmp/shooter.sh & > /dev/null
                fi
        done
else
        echo ....no publicIps found
        exit 1
fi
