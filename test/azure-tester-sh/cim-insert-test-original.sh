RESTSERVER=localhost

 # for Cloud Driver Info
curl -X POST http://$RESTSERVER:1024/driver -H 'Content-Type: application/json' -d '{"DriverName":"azure-driver01","ProviderName":"AZURE", "DriverLibFileName":"azure-driver-v1.0.so"}'

 # for Cloud Credential Info
curl -X POST http://$RESTSERVER:1024/credential -H 'Content-Type: application/json' -d '{"CredentialName":"azure-credential01","ProviderName":"AZURE", "KeyValueInfoList": [{"Key":"ClientId", "Value":"xxxx-xxxx-xxxx-xxxx-xxxx"}, {"Key":"ClientSecret", "Value":"xxxx-xxxx-xxxx-xxxx-xxxx"}, {"Key":"TenantId", "Value":"xxxx-xxxx-xxxx-xxxx-xxxx"}, {"Key":"SubscriptionId", "Value":"xxxx-xxxx-xxxx-xxxx-xxxx"}]}'

 # for Cloud Region Info
curl -X POST http://$RESTSERVER:1024/region -H 'Content-Type: application/json' -d '{"RegionName":"azure-northeu","ProviderName":"AZURE", "KeyValueInfoList": [{"Key":"location", "Value":"northeurope"}, {"Key":"ResourceGroup", "Value":"CB-GROUP-POWERKIM"}]}'

 # for Cloud Connection Config Info
curl -X POST http://$RESTSERVER:1024/connectionconfig -H 'Content-Type: application/json' -d '{"ConfigName":"azure-northeu-config","ProviderName":"AZURE", "DriverName":"azure-driver01", "CredentialName":"azure-credential01", "RegionName":"azure-northeu"}'


