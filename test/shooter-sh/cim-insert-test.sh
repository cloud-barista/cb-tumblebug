RESTSERVER=localhost

## AWS
# for Cloud Driver Info
curl -X POST http://$RESTSERVER:1024/driver -H 'Content-Type: application/json' -d '{"DriverName":"aws-driver01","ProviderName":"AWS", "DriverLibFileName":"aws-driver-v1.0.so"}'

 # for Cloud Credential Info
curl -X POST http://$RESTSERVER:1024/credential -H 'Content-Type: application/json' -d '{"CredentialName":"aws-credential01","ProviderName":"AWS", "KeyValueInfoList": [{"Key":"ClientId", "Value":"xxxxxxxx"}, {"Key":"ClientSecret", "Value":"xxxxx"}]}'

 # Cloud Region Info for Shooter
curl -X POST http://$RESTSERVER:1024/region -H 'Content-Type: application/json' -d '{"RegionName":"aws-ohio","ProviderName":"AWS", "KeyValueInfoList": [{"Key":"Region", "Value":"us-east-2"}]}'
curl -X POST http://$RESTSERVER:1024/region -H 'Content-Type: application/json' -d '{"RegionName":"aws-oregon","ProviderName":"AWS", "KeyValueInfoList": [{"Key":"Region", "Value":"us-west-2"}]}'
curl -X POST http://$RESTSERVER:1024/region -H 'Content-Type: application/json' -d '{"RegionName":"aws-singapore","ProviderName":"AWS", "KeyValueInfoList": [{"Key":"Region", "Value":"ap-southeast-1"}]}'
curl -X POST http://$RESTSERVER:1024/region -H 'Content-Type: application/json' -d '{"RegionName":"aws-paris","ProviderName":"AWS", "KeyValueInfoList": [{"Key":"Region", "Value":"eu-west-3"}]}'
curl -X POST http://$RESTSERVER:1024/region -H 'Content-Type: application/json' -d '{"RegionName":"aws-saopaulo","ProviderName":"AWS", "KeyValueInfoList": [{"Key":"Region", "Value":"sa-east-1"}]}'

 # for test service
curl -X POST http://$RESTSERVER:1024/region -H 'Content-Type: application/json' -d '{"RegionName":"aws-tokyo","ProviderName":"AWS", "KeyValueInfoList": [{"Key":"Region", "Value":"ap-northeast-1"}]}'

 # Cloud Connection Config Info for Shooter
curl -X POST http://$RESTSERVER:1024/connectionconfig -H 'Content-Type: application/json' -d '{"ConfigName":"aws-ohio-config","ProviderName":"AWS", "DriverName":"aws-driver01", "CredentialName":"aws-credential01", "RegionName":"aws-ohio"}'
curl -X POST http://$RESTSERVER:1024/connectionconfig -H 'Content-Type: application/json' -d '{"ConfigName":"aws-oregon-config","ProviderName":"AWS", "DriverName":"aws-driver01", "CredentialName":"aws-credential01", "RegionName":"aws-oregon"}'
curl -X POST http://$RESTSERVER:1024/connectionconfig -H 'Content-Type: application/json' -d '{"ConfigName":"aws-singapore-config","ProviderName":"AWS", "DriverName":"aws-driver01", "CredentialName":"aws-credential01", "RegionName":"aws-singapore"}'
curl -X POST http://$RESTSERVER:1024/connectionconfig -H 'Content-Type: application/json' -d '{"ConfigName":"aws-paris-config","ProviderName":"AWS", "DriverName":"aws-driver01", "CredentialName":"aws-credential01", "RegionName":"aws-paris"}'
curl -X POST http://$RESTSERVER:1024/connectionconfig -H 'Content-Type: application/json' -d '{"ConfigName":"aws-saopaulo-config","ProviderName":"AWS", "DriverName":"aws-driver01", "CredentialName":"aws-credential01", "RegionName":"aws-saopaulo"}'

 # for test service
curl -X POST http://$RESTSERVER:1024/connectionconfig -H 'Content-Type: application/json' -d '{"ConfigName":"aws-tokyo-config","ProviderName":"AWS", "DriverName":"aws-driver01", "CredentialName":"aws-credential01", "RegionName":"aws-tokyo"}'



## Azure
# for Cloud Driver Info
curl -X POST http://$RESTSERVER:1024/driver -H 'Content-Type: application/json' -d '{"DriverName":"azure-driver01","ProviderName":"AZURE", "DriverLibFileName":"azure-driver-v1.0.so"}'

 # for Cloud Credential Info
curl -X POST http://$RESTSERVER:1024/credential -H 'Content-Type: application/json' -d '{"CredentialName":"azure-credential01","ProviderName":"AZURE", "KeyValueInfoList": [{"Key":"ClientId", "Value":"xxxx-xxxx-xxxx-xxxx-xxxx"}, {"Key":"ClientSecret", "Value":"xxxx-xxxx-xxxx-xxxx-xxxx"}, {"Key":"TenantId", "Value":"xxxx-xxxx-xxxx-xxxx-xxxx"}, {"Key":"SubscriptionId", "Value":"xxxx-xxxx-xxxx-xxxx-xxxx"}]}'

 # for Cloud Region Info
curl -X POST http://$RESTSERVER:1024/region -H 'Content-Type: application/json' -d '{"RegionName":"azure-northeu","ProviderName":"AZURE", "KeyValueInfoList": [{"Key":"location", "Value":"northeurope"}, {"Key":"ResourceGroup", "Value":"CB-GROUP-POWERKIM"}]}'

 # for Cloud Connection Config Info
curl -X POST http://$RESTSERVER:1024/connectionconfig -H 'Content-Type: application/json' -d '{"ConfigName":"azure-northeu-config","ProviderName":"AZURE", "DriverName":"azure-driver01", "CredentialName":"azure-credential01", "RegionName":"azure-northeu"}'



## GCP
# for Cloud Driver Info
curl -X POST "http://$RESTSERVER:1024/driver" -H 'Content-Type: application/json' -d '{"DriverName":"gcp-driver01","ProviderName":"GCP", "DriverLibFileName":"gcp-driver-v1.0.so"}'

 # for Cloud Credential Info
curl -X POST "http://$RESTSERVER:1024/credential" -H 'Content-Type: application/json' -d '{"CredentialName":"gcp-credential01","ProviderName":"GCP", "KeyValueInfoList": [{"Key":"PrivateKey", "Value":"-----BEGIN PRIVATE KEY-----\nMIIEvQIBADANB.......\nPxYUOMhvB0nRTX6eEryuwgQ=\n-----END PRIVATE KEY-----\n"},{"Key":"ProjectID", "Value":"powerkim-prj"}, {"Key":"ClientEmail", "Value":"user@user-prj.iam.gserviceaccount.com"}]}'

 # for Cloud Region Info
curl -X POST "http://$RESTSERVER:1024/region" -H 'Content-Type: application/json' -d '{"RegionName":"gcp-taiwan-region","ProviderName":"GCP", "KeyValueInfoList": [{"Key":"Region", "Value":"asia-east1"},{"Key":"Zone", "Value":"asia-east1-a"}]}'

 # for Cloud Connection Config Info
curl -X POST "http://$RESTSERVER:1024/connectionconfig" -H 'Content-Type: application/json' -d '{"ConfigName":"gcp-taiwan-config","ProviderName":"GCP", "DriverName":"gcp-driver01", "CredentialName":"gcp-credential01", "RegionName":"gcp-taiwan-region"}'

