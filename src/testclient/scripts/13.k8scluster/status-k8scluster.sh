#!/bin/bash

echo "####################################################################"
echo "## 13. K8SCLUSTER: Status"
echo "####################################################################"

echo "NOT IMPLEMENTED YET"
exit 1

source ../init.sh

K8SCLUSTERID_ADD=${OPTION03:-1}

K8SCLUSTERID=${K8SCLUSTERID_PREFIX}${INDEX}${REGION}${K8SCLUSTERID_ADD}

echo "NSID: "${NSID}
echo "K8SCLUSTERID=${K8SCLUSTERID}"

GetK8sClusterOption=status
curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NSID/k8scluster/${K8SCLUSTERID}?option=${GetK8sClusterOption} | jq ''

echo -e "${BOLD}"

echo -e "${NC} ${BLUE} ${BOLD}"
curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NSID/k8scluster/${K8SCLUSTERID}?option=${GetK8sClusterOption} |
    jq '.status | .K8SCLUSTER | sort_by(.id)' |
    jq -r '(["NodeGroup-ID","Status","ImageId","SpecId","RootDiskType","RootDiskSize","SshKeyId", "OnAutoScaling", "DesiredNodeSize", "MinNodeSize", "MaxNodeSize"] | (., map(length*"-"))), (.[] | [.id, .status, .imageId, .specId, .rootDiskType, .rootDiskSize, .sshKeyId, .onAutoScaling, .desiredNodeSize, .minNodeSize, .maxNodeSize]) | @tsv' |
    column -t
echo -e "${NC}"

