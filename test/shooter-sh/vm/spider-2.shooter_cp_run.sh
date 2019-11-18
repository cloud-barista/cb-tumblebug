#!/bin/bash
source ../setup.env

KEY_NAME=${CONNECT_NAMES[0]}

num=0
for NAME in "${CONNECT_NAMES[@]}"
do
	if [ $num -eq 0 ] ; then
		echo .... first vm skipped!!
	else
		echo ========================== $NAME
		PUBLIC_IPS=`curl -sX GET http://$RESTSERVER:1024/vm?connection_name=$NAME |json_pp |grep "\"PublicIP\"" |awk '{print $3}' |sed 's/"//g' |sed 's/,//g'`
		for PUBLIC_IP in ${PUBLIC_IPS}
		do
			echo $NAME: copy shooter into ${PUBLIC_IP} ...
			ssh-keygen -f "/root/.ssh/known_hosts" -R ${PUBLIC_IP}
			scp -i ../keypair/$KEY_NAME.key -o "StrictHostKeyChecking no" ./shooter/shooter.sh cb-user@$PUBLIC_IP:/tmp
			ssh -i ../keypair/$KEY_NAME.key -o "StrictHostKeyChecking no" cb-user@$PUBLIC_IP /tmp/shooter.sh &
		done
		
	fi
	num=`expr $num + 1`
done
