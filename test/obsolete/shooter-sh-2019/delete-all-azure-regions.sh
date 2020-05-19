#!/bin/bash
RESTSERVER=localhost

LOCS=(`cat azure-locations-list.txt |grep "name" |awk '{print $2}' |sed 's/",//g' |sed 's/"//g'`)

for LOC in "${LOCS[@]}"
do
	echo $LOC

	curl -sX DELETE http://$RESTSERVER:1024/region/azure-$LOC

done
