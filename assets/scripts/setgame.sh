curl -sL https://deb.nodesource.com/setup_12.x | sudo -E bash -

sudo apt-get update > /dev/null; sudo apt-get -y install nodejs > /dev/null; sudo apt-get -y install git > /dev/null

git clone https://github.com/Jerenaux/westward.git > /dev/null

sudo apt-key adv --keyserver hkp://keyserver.ubuntu.com:80 --recv 9DA31620334BD75D9DCB49F368818C72E52529D4 > /dev/null

echo "deb [ arch=amd64 ] https://repo.mongodb.org/apt/ubuntu bionic/mongodb-org/4.0 multiverse" | sudo tee /etc/apt/sources.list.d/mongodb-org-4.0.list

sudo apt-get update > /dev/null

sudo DEBIAN_FRONTEND=noninteractive apt-get -y install mongodb-org=4.0.5 mongodb-org-server=4.0.5 mongodb-org-shell=4.0.5 mongodb-org-mongos=4.0.5 mongodb-org-tools=4.0.5 > /dev/null

sudo service mongod start

cd ~/westward

sudo npm install > /dev/null

touch .env

sudo nohup node dist/server.js 1>/dev/null 2>&1 &

str=$(curl https://api.ipify.org)

printf "\nGameServer is ready. Access http://%s:8081\n" $str
