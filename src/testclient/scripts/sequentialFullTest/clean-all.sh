
SECONDS=0

./clean-mcis-only.sh "$@"

./clean-mcir-ns-cloud.sh "$@"

duration=$SECONDS
echo "[ElapsedTime] $(($duration / 60)):$(($duration % 60)) (min:sec) $duration (sec) / [Command] $0 "

