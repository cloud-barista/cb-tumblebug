
SECONDS=0

./clean-mcis-only.sh "$@"

./clean-mcir-ns-cloud.sh "$@"

duration=$SECONDS
echo ""
echo "[Command] $0 "
echo "[ElapsedTime] $duration sec  /  $(($duration / 60)) min : $(($duration % 60)) sec"

