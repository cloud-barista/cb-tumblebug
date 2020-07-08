#!/bin/bash
source ../conf.env
source ../credentials.conf
AUTH="Authorization: Basic $(echo -n $ApiUsername:$ApiPassword | base64)"

echo "####################################################################"
echo "## 0. List Cloud Connction Config(s)"
echo "####################################################################"


#INDEX=${1}

RESTSERVER=localhost

# for Cloud Connection Config Info
curl -H "${AUTH}" -sX GET http://$SpiderServer/spider/connectionconfig | json_pp


# for Cloud Region Info
curl -H "${AUTH}" -sX GET http://$SpiderServer/spider/region | json_pp


# for Cloud Credential Info
curl -H "${AUTH}" -sX GET http://$SpiderServer/spider/credential | json_pp

 
# for Cloud Driver Info
curl -H "${AUTH}" -sX GET http://$SpiderServer/spider/driver | json_pp