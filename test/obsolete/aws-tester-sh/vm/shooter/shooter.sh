#!/bin/bash
SERVER=15.222.55.8


HN=`hostname`

while : 
do
	DT=`date`
	DT=`echo $DT |sed 's/ /%20/g'`
	curl -sX GET http://$SERVER:119/test -H 'Content-Type: application/json' -d '{"Date": "'${DT}'", "HostName": "'${HN}'"}'
	sleep 5
done
