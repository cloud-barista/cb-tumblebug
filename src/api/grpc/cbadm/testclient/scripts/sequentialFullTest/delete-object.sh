


    TestSetFile=${4:-../testSet.env}
    if [ ! -f "$TestSetFile" ]; then
        echo "$TestSetFile does not exist."
        exit
    fi
	source $TestSetFile
    source ../conf.env
    
    echo "####################################################################"
    echo "## 0. Object: Delete"
    echo "####################################################################"

    KEY=${1}

    $CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/cbadm util delete-obj --config $CBTUMBLEBUG_ROOT/src/api/grpc/cbadm/grpc_conf.yaml -o json --key $KEY | jq '' 
