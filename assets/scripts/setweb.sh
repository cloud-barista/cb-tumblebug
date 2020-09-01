sudo apt-get update > /dev/null

sudo apt-get -y install nginx > /dev/null

nginx -v

sudo service nginx start

sudo sed -i "s/Welcome to nginx/Welcome, my IP is `curl https://api.ipify.org`/g" /var/www/html/index.nginx-debian.html

sudo sed -i "s/Commercial support is available at/<h2>Check CB-Tumblebug MCIS VM Location<\/h2>/g" /var/www/html/index.nginx-debian.html

sudo sed -i "s/Thank you for using nginx/Thank you for using Cloud-Barista and CB-Tumblebug/g" /var/www/html/index.nginx-debian.html

sudo sed -i "s/<a href=\"http:\/\/nginx.com\/\">nginx.com<\/a>.<\/p>/<a href=\"https:\/\/tools.keycdn.com\/geo?host=`curl https://api.ipify.org`\">Check the Location of NGINX HOST<\/a>.<\/p>/g" /var/www/html/index.nginx-debian.html

sudo sed -i "s/:\/\/nginx.org\/\">nginx.org/s:\/\/cloud-barista.github.io\/\">Cloud-Barista/g" /var/www/html/index.nginx-debian.html

str=$(curl https://api.ipify.org)

printf "WebServer is ready. Access http://%s" $str