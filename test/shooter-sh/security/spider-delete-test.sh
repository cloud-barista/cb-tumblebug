#!/bin/bash
source ../setup.env

#for NAME in "${CONNECT_NAMES[@]}"
#do
	NAME=${CONNECT_NAMES[0]}

        ID=security01-powerkim
        curl -sX DELETE http://$RESTSERVER:1024/securitygroup/${ID}?connection_name=${NAME} &
#done

