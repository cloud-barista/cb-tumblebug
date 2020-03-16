#!/bin/bash
SERVER=35.229.49.221


HN=`hostname`
IP=`curl -s ifconfig.so`
COUNTRY=`curl -sX GET https://api.ipgeolocationapi.com/geolocate/$IP | json_pp |grep "\"name\" :" |awk '{print $3}' | head -n 1 |sed 's/"//g' |sed 's/,//g'`

while : 
do
	DT=`date`
	DT=`echo $DT |sed 's/ /%20/g'`
	curl -sX GET http://$SERVER:119/test -H 'Content-Type: application/json' -d '{
		"Date": "'${DT}'", 
		"HostName": "'${HN}'", 
		"IP": "'$IP'",
		"Country": "'$COUNTRY'"}'
	sleep 5
done
