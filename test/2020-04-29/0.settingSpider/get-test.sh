#!/bin/bash

source ../conf.env

echo "####################################################################"
echo "## 0. Remove Cloud Connction Config"
echo "####################################################################"

INDEX=${1}

RESTSERVER=localhost

# for Cloud Connection Config Info
curl -X GET http://$RESTSERVER:1024/spider/connectionconfig/${CONN_CONFIG[INDEX]} |json_pp


# for Cloud Region Info
curl -X GET http://$RESTSERVER:1024/spider/region/${RegionName[INDEX]} |json_pp


# for Cloud Credential Info
curl -X GET http://$RESTSERVER:1024/spider/credential/${CredentialName[INDEX]} |json_pp

 
# for Cloud Driver Info
curl -X GET http://$RESTSERVER:1024/spider/driver/${DriverName[INDEX]} |json_pp

