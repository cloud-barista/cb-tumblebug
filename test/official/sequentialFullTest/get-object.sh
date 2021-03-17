
    FILE=../conf.env
    if [ ! -f "$FILE" ]; then
        echo "$FILE does not exist."
        exit
    fi

    source ../conf.env
    AUTH="Authorization: Basic $(echo -n $ApiUsername:$ApiPassword | base64)"

    echo "####################################################################"
    echo "## 0. Object: Get value"
    echo "####################################################################"

    KEY=${1}

    curl -H "${AUTH}" -sX GET http://$TumblebugServer/tumblebug/objectValue?key=$KEY | json_pp 
