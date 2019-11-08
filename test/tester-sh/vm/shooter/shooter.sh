#!/bin/bash
SERVER=52.8.176.204


HN=`hostname`

while : 
do
	DT=`date`
	DT=`echo $DT |sed 's/ /%20/g'`
	curl -sX GET http://$SERVER:119/test -H 'Content-Type: application/json' -d '{"Date": "'${DT}'", "HostName": "'${HN}'"}'
	sleep 5
done
