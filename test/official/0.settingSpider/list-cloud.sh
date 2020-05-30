#!/bin/bash
source ../conf.env
source ../credentials.conf

echo "####################################################################"
echo "## 0. List Cloud Connction Config(s)"
echo "####################################################################"


#INDEX=${1}

RESTSERVER=localhost

# for Cloud Connection Config Info
curl -sX GET http://$SpiderServer/spider/connectionconfig | json_pp


# for Cloud Region Info
curl -sX GET http://$SpiderServer/spider/region | json_pp


# for Cloud Credential Info
curl -sX GET http://$SpiderServer/spider/credential | json_pp

 
# for Cloud Driver Info
curl -sX GET http://$SpiderServer/spider/driver | json_pp