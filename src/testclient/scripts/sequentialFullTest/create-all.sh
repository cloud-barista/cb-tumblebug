
SECONDS=0

./create-mcir-ns-cloud.sh "$@"

./create-mcis-only.sh "$@"

duration=$SECONDS
echo "[ElapsedTime] $(($duration / 60)):$(($duration % 60)) (min:sec) $duration (sec) / [Command] $0 "

