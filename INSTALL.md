# Getting started

How to run netlab labs on Kubernetes the way we do: Talos Linux, Cilium, KubeVirt, Multus, then KNE with meshnet, and the patched netlab on your workstation. This repository does not include Skyforge's control plane or portal. This gets you as far as `netlab up` against your cluster.

The versions are what we run. Other versions may work; we haven't tried them.

| Layer | Version |
|---|---|
| Talos Linux / Kubernetes | 1.14 (beta) / 1.36 |
| Cilium | 1.21.0-pre.2 |
| Multus | v4.1.3 |
| KubeVirt / CDI | v1.9.0 / v1.64.0 |
| Gateway API CRDs | v1.5.1 |

Workstation: `talosctl`, `kubectl`, `helm`, Go 1.26+, Docker or another image builder, Python 3.10+.

## 1. Talos

Nodes need hardware virtualisation (`/dev/kvm`) for VM-based NOS images. Build an installer image at [factory.talos.dev](https://factory.talos.dev) with these extensions (we use them for storage):

```yaml
customization:
  systemExtensions:
    officialExtensions:
      - siderolabs/iscsi-tools
      - siderolabs/util-linux-tools
```

Generate the cluster with no default CNI and no kube-proxy (Cilium replaces both), plus a mount for the local-path provisioner:

```yaml
# talos-patch.yaml
cluster:
  network:
    cni: { name: none }
  proxy: { disabled: true }
machine:
  kubelet:
    extraMounts:
      - destination: /var/local-path-provisioner
        type: bind
        source: /var/local-path-provisioner
        options: [bind, rshared, rw]
```

```bash
talosctl gen config lab https://<control-plane>:6443 --config-patch @talos-patch.yaml
# apply controlplane.yaml / worker.yaml to the nodes, then:
talosctl bootstrap -n <control-plane> && talosctl kubeconfig -n <control-plane>
```

## 2. Cluster add-ons

```bash
kubectl apply --server-side -f https://github.com/kubernetes-sigs/gateway-api/releases/download/v1.5.1/standard-install.yaml

helm repo add cilium https://helm.cilium.io
helm install cilium cilium/cilium --version 1.21.0-pre.2 -n kube-system \
  --set kubeProxyReplacement=true \
  --set k8sServiceHost=localhost --set k8sServicePort=7445 \
  --set cni.exclusive=false \
  --set cgroup.autoMount.enabled=false --set cgroup.hostRoot=/sys/fs/cgroup \
  --set securityContext.capabilities.ciliumAgent="{CHOWN,KILL,NET_ADMIN,NET_RAW,IPC_LOCK,SYS_ADMIN,SYS_RESOURCE,DAC_OVERRIDE,FOWNER,SETGID,SETUID}" \
  --set securityContext.capabilities.cleanCiliumState="{NET_ADMIN,SYS_ADMIN,SYS_RESOURCE}" \
  --set ipam.mode=cluster-pool \
  --set "ipam.operator.clusterPoolIPv4PodCIDRList={<pod-cidr>}"

kubectl apply -f https://raw.githubusercontent.com/k8snetworkplumbingwg/multus-cni/v4.1.3/deployments/multus-daemonset.yml

kubectl apply -f https://github.com/kubevirt/kubevirt/releases/download/v1.9.0/kubevirt-operator.yaml
kubectl apply -f https://github.com/kubevirt/kubevirt/releases/download/v1.9.0/kubevirt-cr.yaml
kubectl apply -f https://github.com/kubevirt/containerized-data-importer/releases/download/v1.64.0/cdi-operator.yaml
kubectl apply -f https://github.com/kubevirt/containerized-data-importer/releases/download/v1.64.0/cdi-cr.yaml

# KNE keeps each node's startup config on a PVC in the `local-path` class
kubectl apply -f https://raw.githubusercontent.com/rancher/local-path-provisioner/master/deploy/local-path-storage.yaml
kubectl label ns local-path-storage pod-security.kubernetes.io/enforce=privileged
```

`cni.exclusive=false` matters: Multus and meshnet chain behind Cilium. To use a different storage class, export `KNE_STARTUP_PERSISTENCE_STORAGE_CLASS=<class>` before running netlab. Set it to an empty string to use the cluster default.

## 3. KNE and meshnet

```bash
scripts/apply-patches.sh kne                       # patched tree in .work/kne
cd .work/kne
CGO_ENABLED=0 go build -o ~/bin/kne kne_cli/main.go   # netlab calls `kne`; keep it on PATH

docker build -t <registry>/meshnet:<tag> -f third_party/meshnet/docker/Dockerfile third_party/meshnet
docker push <registry>/meshnet:<tag>
sed "s|us-west1-docker.pkg.dev/kne-external/kne/meshnet:ga|<registry>/meshnet:<tag>|" \
  manifests/meshnet/vxlan/manifest.yaml | kubectl apply -f -
```

We run meshnet in VXLAN mode. Links between nodes are VXLAN over the node network, with a 1450-byte fabric MTU by default (`MESHNET_FABRIC_MTU`).

For Arista cEOS, also build the patched operator and deploy it with KNE's manifest:

```bash
cd .. && ../scripts/apply-patches.sh arista-ceoslab-operator && cd arista-ceoslab-operator
docker build -t <registry>/arista-ceoslab-operator:<tag> . && docker push <registry>/arista-ceoslab-operator:<tag>
sed "s|ghcr.io/aristanetworks/arista-ceoslab-operator:v2.1.2|<registry>/arista-ceoslab-operator:<tag>|" \
  ../kne/manifests/controllers/ceoslab/manifest.yaml | kubectl apply -f -
```

We don't use `kne deploy`. Its MetalLB ingress expects a local Docker network, and our patched KNE gives each lab node a ClusterIP Service, so no load balancer is needed.

## 4. netlab

```bash
scripts/apply-patches.sh netlab
python3 -m venv ~/netlab && . ~/netlab/bin/activate
pip install -e .work/netlab ansible-core passlib
```

The KNE provider is the default. Two settings you will likely need, in the topology or in `~/.netlab.yml` (netlab's user defaults):

```yaml
defaults:
  providers.kne.link_mtu: 1450        # ours is 8950 (jumbo underlay); use 1450 on a 1500-byte network
  kne.image_overrides:                # NOS images are yours to supply; use fully-qualified refs
    eos: <registry>/ceos:<version>
    iol: <registry>/iol:<version>
```

## 5. A first lab

```bash
cd components/blueprints/netlab/BGP/<example>
netlab create        # renders topology.pb.txt for KNE
netlab up            # runs `kne create`, then configures the devices
netlab down
```

Every blueprint here compiles with `netlab create` against the patched netlab. Running one needs images for its devices.
