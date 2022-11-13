sudo apt-get update > /dev/null

sudo apt-get -y install nginx > /dev/null

nginx -v

sudo service nginx start

sudo sed -i "s/<\/title>/<\/title><meta http-equiv=\"refresh\" content=\"1\">/g" /var/www/html/index.nginx-debian.html

sudo sed -i "s/<h1>Welcome to nginx!/<h1><br><br>Welcome to Cloud-Barista<br><br>Host IP is<br>`curl https://api.ipify.org`<br><br>/g" /var/www/html/index.nginx-debian.html

sudo sed -i "s/Commercial support is available at/<h2>Check CB-Tumblebug MCIS VM Location<\/h2>/g" /var/www/html/index.nginx-debian.html

sudo sed -i "s/Thank you for using nginx/Thank you for using Cloud-Barista and CB-Tumblebug/g" /var/www/html/index.nginx-debian.html

sudo sed -i "s/<a href=\"http:\/\/nginx.com\/\">nginx.com<\/a>.<\/p>/<a href=\"https:\/\/www.geolocation.com\/?ip=`curl https://api.ipify.org`#ipresult\">Check the Location of NGINX HOST<\/a>.<\/p>/g" /var/www/html/index.nginx-debian.html

str=$(curl https://api.ipify.org)

printf "WebServer is ready. Access http://%s" $str
