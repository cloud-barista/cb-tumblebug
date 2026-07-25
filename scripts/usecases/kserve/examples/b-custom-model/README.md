# Example B: custom model container on KServe

For models that need custom pre/post-processing code. You write a small
`kserve.Model` class (see `model.py`), build an image, and KServe still provides
the API server, inference protocol, canary rollout, and autoscaling.

Automated path (in-cluster private registry, no external account needed):
run `../build-serve-custom-model.sh` after `../../deploy-private-registry.sh`
and `../../config-registry-access.sh`. The manual path with your own registry:

```bash
# 1. Implement predict() in model.py, then build & push (registry required)
docker build -t <YOUR_REGISTRY>/custom-model:latest .
docker push <YOUR_REGISTRY>/custom-model:latest

# 2. Set the image in isvc.yaml and deploy
kubectl apply -f isvc.yaml
kubectl wait --for=condition=Ready isvc/custom-model --timeout=5m

# 3. Test (V1 inference protocol; input = 8x8 digit image as 64 values)
IP=$(kubectl get svc custom-model-predictor -o jsonpath='{.spec.clusterIP}')
curl -s http://$IP/v1/models/custom-model:predict \
  -H 'Content-Type: application/json' -d '{"instances": [[0,0,5,13,9,1,0,0,0,0,13,15,10,15,5,0,0,3,15,2,0,11,8,0,0,4,12,0,0,8,8,0,0,5,8,0,0,9,8,0,0,4,11,0,1,12,7,0,0,2,14,5,10,12,0,0,0,0,6,13,10,0,0,0]]}'
# → {"predictions": [{"digit": 0, "confidence": 0.999}]}
```
