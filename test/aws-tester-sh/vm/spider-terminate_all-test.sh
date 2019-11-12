#!/bin/bash
source ../setup.env

for NAME in "${CONNECT_NAMES[@]}"
do
	echo ========================== $NAME
	VM_IDS=`curl -sX GET http://$RESTSERVER:1024/vm?connection_name=${NAME}`

	if [ $VM_IDS != "null" ]
	then 
		VM_IDS=`curl -sX GET http://$RESTSERVER:1024/vm?connection_name=${NAME} |json_pp |grep "\"Id\"" |awk '{print $3}' |sed 's/"//g' |sed 's/,//g'`
		for VM_ID in ${VM_IDS}
		do
			echo ....terminate ${VM_ID} ...
			curl -sX DELETE http://$RESTSERVER:1024/vm/${VM_ID}?connection_name=${NAME} 
		done
	else
		echo ....no VMs
	fi
done

