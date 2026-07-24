# KServe LLM Serving on CB-Tumblebug

Serve LLMs with an OpenAI-compatible API on a CB-Tumblebug-provisioned K8s cluster:
**KServe (RawDeployment) + vLLM + Open WebUI**. No Istio/Knative required.

## Prerequisites

- K8s cluster via `../k8s/k8s-control-plane-setup.sh` and `../k8s/k8s-worker-setup.sh`
- GPU worker with NVIDIA driver (GPU-ready image such as Azure `ubuntu-hpc`, or `../llm/installGpuDriver.sh` + reboot)
- Recommended GPU: 24GB+ VRAM (A10G/L4) for 7-8B models

## Steps (all on the control plane)

```bash
# 1. KServe stack: default StorageClass, Helm, GPU Operator, cert-manager, KServe (raw mode)
./deploy-kserve-stack.sh

# 2. Serve a model (OpenAI-compatible API at http://<name>-predictor.default/openai/v1)
./serve-vllm-model.sh --model Qwen/Qwen2.5-7B-Instruct
#   --ctx-len 32768      context length (default: model default)
#   --hf-token <token>   for gated models (Llama, Mistral, ...)
#   --tool-parser        auto-detected from model family (hermes/llama3_json/mistral)
#   --nodeport 30800     expose the API externally (direct API use, Hermes Agent, ...)
#   Re-run with a different --model to swap the served model in place.

# 3. Chat UI on NodePort (open the port in the Security Group)
./deploy-open-webui-kserve.sh --nodeport 30080

# 4. (optional) Hermes Agent on the KServe endpoint
../llm/deployHermesAgent.sh --mode hermes-only --skip-vllm \
  --vllm-base-url http://localhost:30800/openai/v1 --model llm
```

## Notes (lessons baked into these scripts)

- `--enable-auto-tool-choice --tool-call-parser` is required: OpenAI clients like
  Open WebUI send `tool_choice:"auto"`, which vLLM otherwise rejects with 400.
- `deploymentStrategy: Recreate` on the InferenceService: with a single GPU,
  RollingUpdate deadlocks (new pod cannot schedule until the old pod releases the GPU).
- GPU Operator runs with `driver.enabled=false` (driver pre-installed on GPU nodes).
- A default StorageClass is ensured automatically (Open WebUI needs a PVC).
