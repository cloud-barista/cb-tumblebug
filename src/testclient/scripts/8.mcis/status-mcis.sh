#!/bin/bash

#function status_mcis() {

echo "####################################################################"
echo "## 8. VM: Status MCIS"
echo "####################################################################"

source ../init.sh

# if [ "${INDEX}" == "0" ]; then
# 	MCISID=${POSTFIX}
# else
# 	MCISID=${CONN_CONFIG[$INDEX,$REGION]}-${POSTFIX}
# fi

echo "${MCISID}"

GetMCISOption=status
curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NSID/mcis/${MCISID}?option=${GetMCISOption} | jq ''

echo -e "${BOLD}"
echo -e "Table: All VMs in the MCIS : ${MCISID}"

echo -e "${NC} ${BLUE} ${BOLD}"
curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NSID/mcis/${MCISID}?option=${GetMCISOption} |
    jq '.status | .vm | sort_by(.id)' |
    jq -r '(["VM-ID","Status","PublicIP","PrivateIP","CloudType","CloudRegion","CreatedTime"] | (., map(length*"-"))), (.[] | [.id, .status, .publicIp, .privateIp, .location.cloudType, .location.nativeRegion, .createdTime]) | @tsv' |
    column -t
echo -e "${NC}"

#HTTP_CODE=$(curl -o /dev/null -w "%{http_code}\n" -H "${AUTH}" "http://${TumblebugServer}/tumblebug/ns/$NSID/mcis/${MCISID}?option=status" --silent)
#echo "Status: $HTTP_CODE"

#}

#status_mcis
