# Model Serving Examples: three ways to serve a model on K8s

How to serve a user-trained ML model, from lightest to most manual.
All examples run on the cluster built by `../deploy-kserve-stack.sh` (CPU only, no GPU needed).

| | A. KServe + standard runtime | B. KServe + custom container | C. Plain Deployment |
|---|---|---|---|
| When | Model in a standard format (sklearn, xgboost, torch, onnx, ...) | Custom pre/post-processing code needed | Model is already an API-serving container |
| Containerize? | **No** — upload artifact, write 8-line yaml | Yes (kserve SDK wrapper) | Yes (any web framework) |
| You get | Runtime provided, canary, autoscaling, `kubectl get isvc` | Same as A, with your code inside | Full control; scaling/rollout is DIY |
| Files | `a-sklearn-isvc.sh` | `b-custom-model/` | `c-plain-deployment.sh` |

```bash
# A: standard-format model, no image build (public iris model; PVC recipe included for your own)
./a-sklearn-isvc.sh

# B: custom logic wrapped with the kserve SDK — fully automated with the in-cluster registry:
../deploy-private-registry.sh                  # on control plane
../config-registry-access.sh                   # on ALL nodes
./build-serve-custom-model.sh                  # build + push + serve + test
# (manual path with your own registry: see b-custom-model/README.md)

# C: the same model as a plain Deployment + Service, no KServe involved
./c-plain-deployment.sh
```

All of the above are available as steps 10-14 of the "KServe (LLM Serving)"
category in CB-MapUI's remote command popup.
