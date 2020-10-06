
echo "####################################################################"
echo "## Full Test Scripts for CB-Spider IID Working Version - 2020.04.21."
echo "##   4. VM: Terminate(Delete)"
echo "##   3. KeyPair: Delete"
echo "##   4. SecurityGroup: Delete"
echo "##   1. VPC: Delete"
echo "####################################################################"

echo "####################################################################"
echo "## 4. VM: Terminate(Delete)"
echo "####################################################################"
curl -H "${AUTH}" -sX DELETE http://localhost:1024/spider/vm/vm-01 -H 'Content-Type: application/json' -d '{ "ConnectionName": "'${CONN_CONFIG}'"}' |json_pp
echo "============== sleep 15 after delete VM"
sleep 15 

echo "####################################################################"
echo "## 3. KeyPair: Delete"
echo "####################################################################"
curl -H "${AUTH}" -sX DELETE http://localhost:1024/spider/keypair/keypair-01 -H 'Content-Type: application/json' -d '{ "ConnectionName": "'${CONN_CONFIG}'"}' |json_pp
echo "####################################################################"
echo "## 4. SecurityGroup: Delete"
echo "####################################################################"
curl -H "${AUTH}" -sX DELETE http://localhost:1024/spider/securitygroup/sg-01 -H 'Content-Type: application/json' -d '{ "ConnectionName": "'${CONN_CONFIG}'"}' |json_pp
echo "####################################################################"
echo "## 1. VPC: Delete"
echo "####################################################################"
curl -H "${AUTH}" -sX DELETE http://localhost:1024/spider/vpc/vpc-01 -H 'Content-Type: application/json' -d '{ "ConnectionName": "'${CONN_CONFIG}'"}' |json_pp

