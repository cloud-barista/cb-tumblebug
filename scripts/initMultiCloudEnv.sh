#!/bin/bash

if [ -z "$CBTUMBLEBUG_ROOT" ]; then
    SCRIPT_DIR=`dirname ${BASH_SOURCE[0]-$0}`
    export CBTUMBLEBUG_ROOT=`cd $SCRIPT_DIR && cd .. && pwd`
fi

$CBTUMBLEBUG_ROOT/src/testclient/scripts/1.configureSpider/register-cloud-interactive.sh -n tb

$CBTUMBLEBUG_ROOT/src/testclient/scripts/2.configureTumblebug/create-ns.sh -x ns01

echo -e "${BOLD}"
while true; do
    echo "Loading common Specs and Images takes more than 10 minutes."
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
output="$("$CBTUMBLEBUG_ROOT"/src/testclient/scripts/2.configureTumblebug/load-common-resource.sh -n tb)"

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

# Optionally, display failed items
echo "Failed items:"
echo "$output" | grep "\[Failed\]" | while read line; do
    echo "$line" | awk -F"  " '{printf "%-50s %-10s\n", $1, $2}'
done

# Display the counts
echo ""
echo "- Image Success count: $successImageCount (Failed count: $failedImageCount)"
echo "- Spec Success count: $successSpecCount (Failed count: $failedSpecCount)"
echo ""
echo "Total duration: $duration seconds."