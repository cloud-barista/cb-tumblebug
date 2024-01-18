#!/bin/bash

echo "####################################################################"
echo "## 13. CLUSTER: Status"
echo "####################################################################"

echo "NOT IMPLEMENTED YET"
exit 1

source ../init.sh


CLUSTERID=${CLUSTERID_PREFIX}${INDEX}${REGION}

echo "NSID: "${NSID}
echo "CLUSTERID=${CLUSTERID}"

GetClusterOption=status
curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NSID/cluster/${CLUSTERID}?option=${GetClusterOption} | jq ''

echo -e "${BOLD}"

echo -e "${NC} ${BLUE} ${BOLD}"
curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NSID/cluster/${CLUSTERID}?option=${GetClusterOption} |
    jq '.status | .cluster | sort_by(.id)' |
    jq -r '(["NodeGroup-ID","Status","ImageId","SpecId","RootDiskType","RootDiskSize","SshKeyId", "OnAutoScaling", "DesiredNodeSize", "MinNodeSize", "MaxNodeSize"] | (., map(length*"-"))), (.[] | [.id, .status, .imageId, .specId, .rootDiskType, .rootDiskSize, .sshKeyId, .onAutoScaling, .desiredNodeSize, .minNodeSize, .maxNodeSize]) | @tsv' |
    column -t
echo -e "${NC}"

