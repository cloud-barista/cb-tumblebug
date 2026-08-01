#!/bin/bash

# Weave Scope on K8s (run on the control plane) — live cluster topology map for demos.
# Self-contained manifests: the official hosted YAML (cloud.weave.works) is gone
# since the Weaveworks shutdown (2024); images are still published on Docker Hub.
# NOTE: archived project, no security patches. The UI is UNAUTHENTICATED and can
# exec into containers — restrict the NodePort to the demo operator's IP only.

set -e

SCOPE_VERSION="${SCOPE_VERSION:-1.13.2}"
NODEPORT="${NODEPORT:-30040}"
NS="weave"

echo "==== Weave Scope Setup (namespace: ${NS}, NodePort: ${NODEPORT}) ===="

if ! kubectl get nodes > /dev/null 2>&1; then
    echo "ERROR: kubectl cannot reach the cluster. Run this on the control plane."
    exit 1
fi

cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Namespace
metadata:
  name: ${NS}
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: weave-scope
  namespace: ${NS}
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: weave-scope
rules:
  - apiGroups: [""]
    resources: [pods, replicationcontrollers, services, nodes, namespaces, endpoints, persistentvolumes, persistentvolumeclaims]
    verbs: [get, list, watch]
  - apiGroups: [apps]
    resources: [deployments, daemonsets, statefulsets, replicasets]
    verbs: [get, list, watch]
  - apiGroups: [batch]
    resources: [jobs, cronjobs]
    verbs: [get, list, watch]
  - apiGroups: [storage.k8s.io]
    resources: [storageclasses]
    verbs: [get, list, watch]
  - apiGroups: [""]
    resources: [pods/log]
    verbs: [get]
  - apiGroups: [""]
    resources: [pods/exec]
    verbs: [create]
  - apiGroups: [apps]
    resources: [deployments/scale]
    verbs: [get, update]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: weave-scope
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: weave-scope
subjects:
  - kind: ServiceAccount
    name: weave-scope
    namespace: ${NS}
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: scope-app
  namespace: ${NS}
spec:
  replicas: 1
  selector:
    matchLabels: { app: scope-app }
  template:
    metadata:
      labels: { app: scope-app }
    spec:
      serviceAccountName: weave-scope
      containers:
        - name: app
          image: weaveworks/scope:${SCOPE_VERSION}
          args: ["--mode=app"]
          ports:
            - containerPort: 4040
          readinessProbe:
            httpGet: { path: /, port: 4040 }
            initialDelaySeconds: 5
            periodSeconds: 5
---
apiVersion: v1
kind: Service
metadata:
  name: scope-app
  namespace: ${NS}
spec:
  type: NodePort
  selector: { app: scope-app }
  ports:
    - port: 4040
      targetPort: 4040
      nodePort: ${NODEPORT}
---
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: scope-agent
  namespace: ${NS}
spec:
  selector:
    matchLabels: { app: scope-agent }
  template:
    metadata:
      labels: { app: scope-agent }
    spec:
      serviceAccountName: weave-scope
      hostPID: true
      hostNetwork: true
      dnsPolicy: ClusterFirstWithHostNet
      tolerations:
        - operator: Exists
      containers:
        - name: agent
          image: weaveworks/scope:${SCOPE_VERSION}
          args:
            - "--mode=probe"
            - "--probe.kubernetes=true"
            - "--probe.kubernetes.role=host"
            - "--probe.docker=false"
            # containerd clusters: fill the Containers view via the CRI probe
            - "--probe.cri=true"
            - "--probe.cri.endpoint=unix:///run/containerd/containerd.sock"
            - "scope-app.${NS}.svc.cluster.local:4040"
          securityContext:
            privileged: true
          volumeMounts:
            - name: cri-sock
              mountPath: /run/containerd/containerd.sock
          resources:
            requests: { cpu: 50m, memory: 100Mi }
      volumes:
        - name: cri-sock
          hostPath:
            path: /run/containerd/containerd.sock
            type: Socket
---
# Cluster-scope probe: watches the K8s API and feeds the Pods/Services/
# Deployments topologies (host-role agents only report node-local data)
apiVersion: apps/v1
kind: Deployment
metadata:
  name: scope-cluster-agent
  namespace: ${NS}
spec:
  replicas: 1
  selector:
    matchLabels: { app: scope-cluster-agent }
  template:
    metadata:
      labels: { app: scope-cluster-agent }
    spec:
      serviceAccountName: weave-scope
      containers:
        - name: agent
          image: weaveworks/scope:${SCOPE_VERSION}
          args:
            - "--mode=probe"
            - "--probe.kubernetes=true"
            - "--probe.kubernetes.role=cluster"
            - "--probe.docker=false"
            - "scope-app.${NS}.svc.cluster.local:4040"
          resources:
            requests: { cpu: 50m, memory: 100Mi }
EOF

echo ""
echo "Waiting for Weave Scope to start..."
kubectl -n ${NS} rollout status deployment/scope-app --timeout=3m > /dev/null
kubectl -n ${NS} rollout status daemonset/scope-agent --timeout=3m > /dev/null
kubectl -n ${NS} rollout status deployment/scope-cluster-agent --timeout=3m > /dev/null

NODE_IP=$(hostname -I | awk '{print $1}')
echo ""
echo "=== Smoke test via NodePort ==="
curl -fsS -o /dev/null -w "HTTP %{http_code}\n" "http://${NODE_IP}:${NODEPORT}/" || true

echo ""
echo "========================================"
echo "SUCCESS: Weave Scope is ready"
echo "========================================"
echo ""
echo "SECURITY: the UI has NO authentication and includes container exec/stop"
echo "controls — open NodePort ${NODEPORT} in the security group for YOUR IP only,"
echo "and remove it (or run the uninstall below) right after the demo."
echo ""
echo "Uninstall: kubectl delete namespace ${NS}; kubectl delete clusterrole,clusterrolebinding weave-scope"
echo ""
echo "\$\$CMD[Check Scope pods](kubectl get pods -n ${NS} -o wide)"
exit 0
