
SECONDS=0

./create-mcir-ns-cloud.sh "$@"

./create-mcis-only.sh "$@"

duration=$SECONDS

source ../common-functions.sh
printElapsed $@
echo "" >>./executionStatus.history
