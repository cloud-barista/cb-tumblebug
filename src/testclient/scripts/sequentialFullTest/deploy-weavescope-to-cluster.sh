#!/bin/bash

# From https://aweirdimagination.net/2020/06/28/kill-child-jobs-on-script-exit/
cleanup() {
    # kill all processes whose parent is this process
    pkill -P $$
}

for sig in INT QUIT HUP TERM; do
  trap "
    cleanup
    trap - $sig EXIT
    kill -s $sig "'"$$"' "$sig"
done
trap cleanup EXIT

# From https://www.grepper.com/answers/215915/regex+for+url+in+bash
readonly ENDPOINT_REGEX='^[-A-Za-z0-9\+&@#/%?=~_|!:,.;]*[-A-Za-z0-9\+&@#/%=~_|]\.[-A-Za-z0-9\+&@#/%?=~_|!:,.;]*[-A-Za-z0-9\+&@#/%=~_|]$'

SECONDS=0

echo "####################################################################"
echo "## Command (SSH) to Cluster (deploy-weavescope-to-cluster)"
echo "####################################################################"

source ../init.sh

KEEP_PREV_KUBECONFIG=${OPTION02:-n}

KUBECTL=kubectl
if ! kubectl > /dev/null 2>&1; then
    	if ! ./kubectl > /dev/null 2>&1; then
		# Download kubectl    
		curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl"
		chmod u+x ./kubectl
	fi
	KUBECTL=./kubectl
fi

if [ "${INDEX}" == "0" ]; then
	INDEXX=${NumCSP}
	for ((cspi = 1; cspi <= INDEXX; cspi++)); do
		INDEXY=${NumRegion[$cspi]}
		CSP=${CSPType[$cspi]}
		for ((cspj = 1; cspj <= INDEXY; cspj++)); do
			REGION=$cspj

			CLUSTERID=${CLUSTERID_PREFIX}${cspi}${cspj}

			echo "[Get ClusterInfo for ${CLUSTERID}]"
			CLUSTERINFO=$(curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NSID/cluster/${CLUSTERID})

			TMP_FILE_KUBECONFIG=$(mktemp ./${CLUSTERID}-kubeconfig.XXXXXX || exit 1)
			if [ "${KEEP_PREV_KUBECONFIG}" != "y" ]; then
			    	echo "Delete Previous Kubeconfig Files"
				rm -f "${CLUSTERID}-kubeconfig."*
				#trap 'echo "trapped"; rm -f -- "$TMP_FILE_KUBECONFIG"' EXIT
			fi

			ENDPOINT=$(jq -r '.AccessInfo.endpoint' <<<"$CLUSTERINFO")
			if [[ ! $ENDPOINT =~ $ENDPOINT_REGEX ]]; then
				echo ".AccessInfo.endpoint ($ENDPOINT) is not valid"	
				echo "Try again after about 5 minutes"		
				break
			fi

			echo "TMP_FILE_KUBECONFIG="$TMP_FILE_KUBECONFIG
			jq -r '.AccessInfo.kubeconfig' <<<"$CLUSTERINFO" > $TMP_FILE_KUBECONFIG
			$KUBECTL --kubeconfig $TMP_FILE_KUBECONFIG apply -f https://github.com/weaveworks/scope/releases/download/v1.13.2/k8s-scope.yaml
			dozing 30

			# max(cspi)=17, max(cspj)=40
			LOCALPORT=$((4000+$cspi*64+$cspj))
			echo "LOCALPORT="$LOCALPORT
			$KUBECTL --kubeconfig $TMP_FILE_KUBECONFIG port-forward --address=0.0.0.0 -n weave "$($KUBECTL --kubeconfig $TMP_FILE_KUBECONFIG get -n weave pod --selector=weave-scope-component=app -o jsonpath='{.items..metadata.name}')" $LOCALPORT:4040 &

			echo "[Cluster Weavescope: complete to creating cluster in $CSP[$REGION]]"
			echo "You can access to http://localhost:"$LOCALPORT "until exiting by Ctrl+C"
		 done
	done
	wait

fi

echo "Done!"
duration=$SECONDS

printElapsed $@
echo ""

