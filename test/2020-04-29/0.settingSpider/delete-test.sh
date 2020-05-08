#!/bin/bash

source ../conf.env

echo "####################################################################"
echo "## 0. Remove All Cloud Connction Config(s)"
echo "####################################################################"

INDEX=${1}

RESTSERVER=localhost

# for Cloud Connection Config Info
curl -sX DELETE http://$RESTSERVER:1024/spider/connectionconfig/${CONN_CONFIG[INDEX]}

# for Cloud Region Info
curl -sX DELETE http://$RESTSERVER:1024/spider/region/${RegionName[INDEX]}

# for Cloud Credential Info
curl -sX DELETE http://$RESTSERVER:1024/spider/credential/${CredentialName[INDEX]}
 
# for Cloud Driver Info
curl -sX DELETE http://$RESTSERVER:1024/spider/driver/${DriverName[INDEX]}
