#!/bin/bash

# CB-Tumblebug API Server
TumblebugServer=localhost:1323

# CB-Tumblebug API BasicAuth Header
ApiUsername=default
ApiPassword=default
AUTH="Authorization: Basic $(echo -n $ApiUsername:$ApiPassword | base64)"

if [ -z "$CBTUMBLEBUG_ROOT" ]; then
    SCRIPT_DIR=`dirname ${BASH_SOURCE[0]-$0}`
    export CBTUMBLEBUG_ROOT=`cd $SCRIPT_DIR && cd .. && pwd`
fi

CRED_FILE_NAME="credentials.yaml"
CRED_PATH="$HOME/.cloud-barista"
if [ ! -d "$CRED_PATH" ]; then
    echo "Error: $CRED_PATH does not exist. Please run scripts/genCredential.sh first."
    exit 1
elif [ ! -f "$CRED_PATH/$CRED_FILE_NAME" ]; then
    echo "Error: $CRED_PATH/$CRED_FILE_NAME does not exist. Please check if it has been generated."
    exit 1
fi

# Load credentials from YAML file
declare -A credentials
mapfile -t csp_list < <(yq eval '.credentialholder.admin | keys' $CRED_PATH/$CRED_FILE_NAME)

# Register credentials to TumblebugServer
for csp in "${csp_list[@]}"; do
    echo "Processing $csp credentials..."
    # Check if all required keys have values
    filled=$(yq eval-all '. as $item ireduce ({}; . * $item)' $CRED_PATH/$CRED_FILE_NAME -j | jq -r --arg CSP "$csp" '.credentialholder.admin[$CSP] | all(. != null and . != "")')
    if [[ $filled == "true" ]]; then
        # Read and prepare credential data
        credential_data=$(yq eval -o=json ".credentialholder.admin.$csp" $CRED_PATH/$CRED_FILE_NAME)
        keyValueInfoList=$(echo $credential_data | jq 'to_entries | map({key, value: .value})')

        # Post credentials
        resp=$(curl -H "${AUTH}" -sX POST "http://$TumblebugServer/tumblebug/credential" -H 'Content-Type: application/json' -d @- <<EOF
{
    "credentialHolder": "admin",
    "keyValueInfoList": $keyValueInfoList,
    "providerName": "$csp"
}
EOF
)
        echo ${resp} | jq ''
    else
        echo "Incomplete credentials for $csp, skipping..."
    fi
done

echo "Credential registration completed."

echo -e "${BOLD}"
while true; do
    echo "Loading common Specs and Images takes time."
    read -p 'Load common Specs and Images. Do you want to proceed ? (y/n) : ' CHECKPROCEED
    echo -e "${NC}"
    case $CHECKPROCEED in
    [Yy]*)
        break
        ;;
    [Nn]*)
        echo
        echo "Cancel [$0 $@]"
        echo "See you soon. :)"
        echo
        exit 1
        ;;
    *)
        echo "Please answer yes or no."
        ;;
    esac
done



# Start time
start_time=$(date +%s)


# Execute the load-common-resource script and capture its output
EXPECTED_DURATION=240 # 4 minutes
progress_time=$(date +%s)

curl -H "${AUTH}" -sX GET http://"$TumblebugServer"/tumblebug/loadCommonResource | jq '' > initTmp.out &
PID=$!

# Initialize the progress bar
progress=0
progress_max=50
printf "["
printf "%-${progress_max}s" "-" | tr " " "-"
printf "]"

while kill -0 $PID 2> /dev/null; do
    current_time=$(date +%s)
    elapsed=$((current_time - progress_time))
    progress=$((elapsed * 100 / EXPECTED_DURATION))

    # Calculate remaining time
    remain=$((EXPECTED_DURATION - elapsed))
    remain_min=$((remain / 60))
    remain_sec=$((remain % 60))

    # Clear the current line
    printf "\r"

    # Reprint the progress bar with current progress
    printf "["
    cur_progress=$((progress * progress_max / 100))
    cur_progress=$((cur_progress>progress_max?progress_max:cur_progress)) # Ensure current progress does not exceed max
    printf "%-${cur_progress}s" "#" | tr " " "#"
    printf "%-$((progress_max - cur_progress))s" " " | tr " " " "
    printf "]"

    # Print the remaining time or the overtime on the right without affecting the progress bar's length
    if [ $remain -lt 0 ]; then
        # If over the expected time, display in negative
        printf " -%02d:%02d overtime" $((-remain_min)) $((-remain_sec))
    else
        printf " %02d:%02d " $remain_min $remain_sec
    fi

    sleep 1
done


echo ""
echo ""
echo "Done"
output=$(<initTmp.out)
rm initTmp.out

# End time
end_time=$(date +%s)
# Calculate duration
duration=$((end_time - start_time))

# Initialize counters
successImageCount=0
failedImageCount=0
successSpecCount=0
failedSpecCount=0

# Count successes and failures for images and specs
while IFS= read -r line; do
    if [[ $line == *"image:"* ]]; then
        if [[ $line == *"[Failed]"* ]]; then
            ((failedImageCount++))
        else
            ((successImageCount++))
        fi
    elif [[ $line == *"spec:"* ]]; then
        if [[ $line == *"[Failed]"* ]]; then
            ((failedSpecCount++))
        else
            ((successSpecCount++))
        fi
    fi
done <<< "$output"

# # Optionally, display failed items
# echo ""
# echo "Failed items:"
# echo "$output" | grep "\[Failed\]" | while read line; do
#     echo "$line" | awk -F"  " '{printf "%-50s %-10s\n", $1, $2}'
# done

# Display the counts
echo ""
echo "- Image Success count: $successImageCount (not registered: $failedImageCount)"
echo "- Spec Success count: $successSpecCount (not registered: $failedSpecCount)"
echo ""
echo "Total duration: $duration seconds."