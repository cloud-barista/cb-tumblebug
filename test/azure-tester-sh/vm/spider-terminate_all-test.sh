#!/bin/bash
source ../setup.env

num=0
for NAME in "${CONNECT_NAMES[@]}"
do
        echo ========================== $NAME

	VM_ID=jhseo-test
	echo ....terminate ${VM_ID} ...
	curl -sX DELETE http://$RESTSERVER:1024/vm/${VM_ID}?connection_name=${NAME} &

	num=`expr $num + 1`
done

