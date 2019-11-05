RESTSERVER=192.168.0.102
 # for Cloud Driver Info
#curl -X POST http://$RESTSERVER:1024/driver -H 'Content-Type: application/json' -d '{"DriverName":"aws-driver01","ProviderName":"AWS", "DriverLibFileName":"aws-driver-v1.0.so"}'

 # for Cloud Credential Info
#curl -X POST http://$RESTSERVER:1024/credential -H 'Content-Type: application/json' -d '{"CredentialName":"aws-credential01","ProviderName":"AWS", "KeyValueInfoList": [{"Key":"ClientId", "Value":"xxxxxxxx"}, {"Key":"ClientSecret", "Value":"xxxxx"}]}'

 # Cloud Region Info for Shooter
#curl -X POST http://$RESTSERVER:1024/region -H 'Content-Type: application/json' -d '{"RegionName":"aws-ohio","ProviderName":"AWS", "KeyValueInfoList": [{"Key":"Region", "Value":"us-east-2"}]}'
#curl -X POST http://$RESTSERVER:1024/region -H 'Content-Type: application/json' -d '{"RegionName":"aws-oregon","ProviderName":"AWS", "KeyValueInfoList": [{"Key":"Region", "Value":"us-west-2"}]}'
#curl -X POST http://$RESTSERVER:1024/region -H 'Content-Type: application/json' -d '{"RegionName":"aws-singapore","ProviderName":"AWS", "KeyValueInfoList": [{"Key":"Region", "Value":"ap-southeast-1"}]}'
#curl -X POST http://$RESTSERVER:1024/region -H 'Content-Type: application/json' -d '{"RegionName":"aws-paris","ProviderName":"AWS", "KeyValueInfoList": [{"Key":"Region", "Value":"eu-west-3"}]}'
#curl -X POST http://$RESTSERVER:1024/region -H 'Content-Type: application/json' -d '{"RegionName":"aws-saopaulo","ProviderName":"AWS", "KeyValueInfoList": [{"Key":"Region", "Value":"sa-east-1"}]}'
curl -X POST http://$RESTSERVER:1024/region -H 'Content-Type: application/json' -d '{"RegionName":"aws-canada-central","ProviderName":"AWS", "KeyValueInfoList": [{"Key":"Region", "Value":"ca-central-1"}]}'

 # for test service
curl -X POST http://$RESTSERVER:1024/region -H 'Content-Type: application/json' -d '{"RegionName":"aws-california-north","ProviderName":"AWS", "KeyValueInfoList": [{"Key":"Region", "Value":"us-west-1"}]}'

 # Cloud Connection Config Info for Shooter
#curl -X POST http://$RESTSERVER:1024/connectionconfig -H 'Content-Type: application/json' -d '{"ConfigName":"aws-ohio-config","ProviderName":"AWS", "DriverName":"aws-driver01", "CredentialName":"aws-credential01", "RegionName":"aws-ohio"}'
#curl -X POST http://$RESTSERVER:1024/connectionconfig -H 'Content-Type: application/json' -d '{"ConfigName":"aws-oregon-config","ProviderName":"AWS", "DriverName":"aws-driver01", "CredentialName":"aws-credential01", "RegionName":"aws-oregon"}'
#curl -X POST http://$RESTSERVER:1024/connectionconfig -H 'Content-Type: application/json' -d '{"ConfigName":"aws-singapore-config","ProviderName":"AWS", "DriverName":"aws-driver01", "CredentialName":"aws-credential01", "RegionName":"aws-singapore"}'
#curl -X POST http://$RESTSERVER:1024/connectionconfig -H 'Content-Type: application/json' -d '{"ConfigName":"aws-paris-config","ProviderName":"AWS", "DriverName":"aws-driver01", "CredentialName":"aws-credential01", "RegionName":"aws-paris"}'
#curl -X POST http://$RESTSERVER:1024/connectionconfig -H 'Content-Type: application/json' -d '{"ConfigName":"aws-saopaulo-config","ProviderName":"AWS", "DriverName":"aws-driver01", "CredentialName":"aws-credential01", "RegionName":"aws-saopaulo"}'
curl -X POST http://$RESTSERVER:1024/connectionconfig -H 'Content-Type: application/json' -d '{"ConfigName":"aws-canada-central-config","ProviderName":"AWS", "DriverName":"aws-driver01", "CredentialName":"aws-credential01", "RegionName":"aws-canada-central"}'

 # for test service
curl -X POST http://$RESTSERVER:1024/connectionconfig -H 'Content-Type: application/json' -d '{"ConfigName":"aws-california-north-config","ProviderName":"AWS", "DriverName":"aws-driver01", "CredentialName":"aws-credential01", "RegionName":"aws-california-north"}'

