RESTSERVER=localhost

curl -X POST "http://$RESTSERVER:1024/driver" -H 'Content-Type: application/json' -d '{"DriverName":"gcp-driver01","ProviderName":"GCP", "DriverLibFileName":"gcp-driver-v1.0.so"}'

 # for Cloud Credential Info
curl -X POST "http://$RESTSERVER:1024/credential" -H 'Content-Type: application/json' -d '{"CredentialName":"gcp-credential01","ProviderName":"GCP", "KeyValueInfoList": [{"Key":"PrivateKey", "Value":"-----BEGIN PRIVATE KEY-----\nMIIEvQIBADANB.......\nPxYUOMhvB0nRTX6eEryuwgQ=\n-----END PRIVATE KEY-----\n"},{"Key":"ProjectID", "Value":"powerkim-prj"}, {"Key":"ClientEmail", "Value":"user@user-prj.iam.gserviceaccount.com"}]}'

 # for Cloud Region Info
curl -X POST "http://$RESTSERVER:1024/region" -H 'Content-Type: application/json' -d '{"RegionName":"gcp-taiwan-region","ProviderName":"GCP", "KeyValueInfoList": [{"Key":"Region", "Value":"asia-east1"},{"Key":"Zone", "Value":"asia-east1-a"}]}'

 # for Cloud Connection Config Info
curl -X POST "http://$RESTSERVER:1024/connectionconfig" -H 'Content-Type: application/json' -d '{"ConfigName":"gcp-taiwan-config","ProviderName":"GCP", "DriverName":"gcp-driver01", "CredentialName":"gcp-credential01", "RegionName":"gcp-taiwan-region"}'

