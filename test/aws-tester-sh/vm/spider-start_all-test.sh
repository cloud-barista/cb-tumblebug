#!/bin/bash
source ../setup.env

num=0
for NAME in "${CONNECT_NAMES[@]}"
do
	echo ========================== $NAME
	VNET_ID=`curl -sX GET http://$RESTSERVER:1024/vnetwork?connection_name=${NAME} |json_pp |grep "\"Id\"" |awk '{print $3}' |sed 's/"//g' |sed 's/,//g'`
	PIP_ID=`curl -sX GET http://$RESTSERVER:1024/publicip?connection_name=${NAME} |json_pp |grep "\"Name\" :" |awk '{print $3}' | head -n 1 |sed 's/"//g' |sed 's/,//g'`
	SG_ID1=` curl -sX GET http://$RESTSERVER:1024/securitygroup?connection_name=${NAME} |json_pp |grep "\"Id\" :" |awk '{print $3}' | head -n 1 |sed 's/"//g' |sed 's/,//g'`
	SG_ID2=`curl -sX GET http://$RESTSERVER:1024/securitygroup?connection_name=${NAME} |json_pp |grep "\"Id\" :" |awk '{print $3}' |awk '{if(NR==2) print $1}' |sed 's/"//g' |sed 's/,//g'`
	VNIC_ID=`curl -sX GET http://$RESTSERVER:1024/vnic?connection_name=${NAME} |json_pp |grep "eni" |awk '{print $3}' |sed 's/"//g' |sed 's/,//g'`

	#echo ${VNET_ID}, ${PIP_ID}, ${SG_ID}, ${VNIC_ID}

	curl -sX POST http://$RESTSERVER:1024/vm?connection_name=${NAME} -H 'Content-Type: application/json' -d '{
	    "VMName": "vm-powerkim01",
		"ImageId": "'${IMG_IDS[num]}'",
		"VirtualNetworkId": "'${VNET_ID}'",
		"NetworkInterfaceId": "'${VNIC_ID}'",
		"PublicIPId": "'${PIP_ID}'",
	    "SecurityGroupIds": [
		"'${SG_ID1}'",  "'${SG_ID2}'"
	    ],
		"VMSpecId": "t2.micro",
		"KeyPairName": "mcb-keypair-powerkim",
		"VMUserId": "",
		"VMUserPasswd": ""
	}' |json_pp &


	num=`expr $num + 1`
done
