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
echo "## Command (SSH) to K8sCluster (deploy-nginx-with-pvc-to-k8scluster)"
echo "####################################################################"

source ../init.sh

KEEP_PREV_KUBECONFIG=${OPTION02:-n}
K8SCLUSTERID_ADD=${OPTION03:-1}
LOCALIP=`hostname -I | cut -d' ' -f1`

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

			K8SCLUSTERID=${K8SCLUSTERID_PREFIX}${cspi}${cspj}${K8SCLUSTERID_ADD}

			echo "[Get K8sClusterInfo for ${K8SCLUSTERID}]"
			K8SCLUSTERINFO=$(curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/ns/$NSID/k8scluster/${K8SCLUSTERID})

			TMP_FILE_KUBECONFIG=$(mktemp ./${K8SCLUSTERID}-kubeconfig.XXXXXX || exit 1)
			if [ "${KEEP_PREV_KUBECONFIG}" != "y" ]; then
			    	echo "Delete Previous Kubeconfig Files"
				rm -f "${K8SCLUSTERID}-kubeconfig."*
				#trap 'echo "trapped"; rm -f -- "$TMP_FILE_KUBECONFIG"' EXIT
			fi

			ENDPOINT=$(jq -r '.AccessInfo.endpoint' <<<"$K8SCLUSTERINFO")
			if [[ ! $ENDPOINT =~ $ENDPOINT_REGEX ]]; then
				echo ".AccessInfo.endpoint ($ENDPOINT) is not valid"	
				echo "Try again after about 5 minutes"		
				continue
			fi

			echo "TMP_FILE_KUBECONFIG="$TMP_FILE_KUBECONFIG
			jq -r '.AccessInfo.kubeconfig' <<<"$K8SCLUSTERINFO" > $TMP_FILE_KUBECONFIG
			$KUBECTL --kubeconfig $TMP_FILE_KUBECONFIG apply -f ./nginx-with-pvc.yaml
			dozing 1

			# max(cspi)=17, max(cspj)=40
			LOCALPORT=$((4000+$cspi*64+$cspj))
			echo "LOCALPORT="$LOCALPORT
			$KUBECTL --kubeconfig $TMP_FILE_KUBECONFIG port-forward --address=0.0.0.0 pod/nginx $LOCALPORT:80 &

			echo "[K8sCluster Nginx for PV/PVC Test: complete to create a k8scluster in $CSP[$REGION]]"
			echo "You can access to http://"$LOCALIP":"$LOCALPORT "until exiting by Ctrl+C"

			$KUBECTL --kubeconfig $TMP_FILE_KUBECONFIG get pv,pvc &
		 done
	done
	wait

fi

echo "Done!"
duration=$SECONDS

printElapsed $@
echo ""

