
SECONDS=0

./create-mcir-ns-cloud.sh "$@"

./create-mcis-only.sh "$@"

duration=$SECONDS
echo ""
echo "[Command] $0 "
echo "[ElapsedTime] $duration sec  /  $(($duration / 60)) min : $(($duration % 60)) sec"

