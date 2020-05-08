#!/bin/bash

source ../conf.env

echo "####################################################################"
echo "## 0. Remove Cloud Connction Config"
echo "####################################################################"

INDEX=${1}

RESTSERVER=localhost

# for Cloud Connection Config Info
curl -X DELETE http://$RESTSERVER:1024/spider/connectionconfig/${CONN_CONFIG[INDEX]}

# for Cloud Region Info
curl -X DELETE http://$RESTSERVER:1024/spider/region/${RegionName[INDEX]}

# for Cloud Credential Info
curl -X DELETE http://$RESTSERVER:1024/spider/credential/${CredentialName[INDEX]}
 
# for Cloud Driver Info
curl -X DELETE http://$RESTSERVER:1024/spider/driver/${DriverName[INDEX]}
