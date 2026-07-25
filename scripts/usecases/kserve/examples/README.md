# One ML task, three serving methods

All three examples serve the **same handwritten-digit recognition model**
(sklearn digits, 8x8 images) — so the only difference you see is the serving
method itself. All run on the cluster built by `../deploy-kserve-stack.sh`
(CPU only, no GPU needed).

| | A. KServe standard runtime | B. KServe custom container | C. Plain Deployment |
|---|---|---|---|
| Serving code you write | **none** | predict() ~20 lines | full API server |
| Image build | **no** | yes (registry needed) | yes (or inline hack) |
| Scaling / rollout / canary | KServe | KServe | **manual (kubectl)** |
| When to choose | model in a standard format | custom pre/post-processing | already have an API container |
| Demo highlight | train Job → PVC → 8-line YAML | image from in-cluster private registry, digit + confidence | scale 1→3, load-balancing bar chart |

```bash
# A: train in-cluster, serve the artifact with zero code
./a-sklearn-isvc.sh

# B: custom predict() → build → push to in-cluster registry → serve
../deploy-private-registry.sh                  # on control plane
../config-registry-access.sh                   # on ALL nodes
./build-serve-custom-model.sh
# (manual path with your own registry: see b-custom-model/README.md)

# C: the same model behind a hand-written FastAPI + manual scaling
./c-plain-deployment.sh
```

All of the above are available as steps 10-14 of the "KServe (LLM Serving)"
category in CB-MapUI's remote command popup.
