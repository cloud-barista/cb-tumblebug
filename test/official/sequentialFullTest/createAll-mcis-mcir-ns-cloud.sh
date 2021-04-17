
SECONDS=0

./prepare-mcir-ns-cloud.sh "$@"

./create-mcis-only.sh "$@"

duration=$SECONDS
echo "$(($duration / 60)) minutes and $(($duration % 60)) seconds elapsed."

echo ""
echo "[Executed Command List]"
cat ./executionStatus
echo ""
