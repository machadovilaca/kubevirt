# GPU Metrics Manual Test Setup

This guide walks through manually testing the GPU metrics collection pipeline
using kubevirtci and the gpu-metrics-agent.

## Prerequisites

- A running kubevirtci cluster with KubeVirt deployed (`make cluster-up && make cluster-sync`)
- The `gpu-metrics-agent` source code at `../gpu-metrics-agent/` (relative to the kubevirt repo)
- `virtctl`, `kubectl`, and `sshpass` available on your PATH
- A guest VM (GPU not required for basic channel testing; NVIDIA GPU needed for real metrics)

## 1. Build the gpu-metrics-agent

Build the agent for Linux (requires CGO for NVML C bindings):

```bash
cd ../gpu-metrics-agent
make build
cd -
```

For Windows guests, cross-compile instead:

```bash
cd ../gpu-metrics-agent
make build-windows
cd -
```

## 2. Create the VMI

```bash
kubectl apply -f - <<'EOF'
apiVersion: kubevirt.io/v1
kind: VirtualMachineInstance
metadata:
  labels:
    special: vmi-fedora
  name: vmi-fedora
spec:
  domain:
    devices:
      disks:
      - disk:
          bus: virtio
        name: containerdisk
      - disk:
          bus: virtio
        name: cloudinitdisk
      interfaces:
      - masquerade: {}
        name: default
      rng: {}
    memory:
      guest: 1024M
    resources: {}
  networks:
  - name: default
    pod: {}
  terminationGracePeriodSeconds: 0
  volumes:
  - containerDisk:
      image: registry:5000/kubevirt/fedora-with-test-tooling-container-disk:devel
    name: containerdisk
  - cloudInitNoCloud:
      userData: |-
        #cloud-config
        password: fedora
        chpasswd: { expire: False }
        ssh_pwauth: true
    name: cloudinitdisk
EOF
```

Wait for the VMI to be running:

```bash
kubectl wait vmi vmi-fedora --for=jsonpath='{.status.phase}'=Running --timeout=120s
```

The GPU metrics virtio-serial channel (`org.kubevirt.gpu-metrics.0`) is
automatically configured for all VMIs (temporarily, for testing; will be
gated behind `spec.domain.devices.gpus` later).

## 3. Copy the Agent into the Guest

Wait ~30 seconds for the guest SSH to be ready, then copy the binary:

### Linux guest

```bash
sshpass -p fedora virtctl scp \
  -t "-o StrictHostKeyChecking=no" \
  -t "-o UserKnownHostsFile=/dev/null" \
  -t "-o PreferredAuthentications=password" \
  -t "-o PubkeyAuthentication=no" \
  ../gpu-metrics-agent/gpu-metrics-agent fedora@vmi-fedora:/home/fedora/
```

### Windows guest

Copy `../gpu-metrics-agent/gpu-metrics-agent.exe` into the guest using
WinRM, RDP shared folder, or another file transfer method.

## 4. Start the Agent

The agent requires root to access the virtio-serial device.

### Linux guest

```bash
sshpass -p fedora virtctl ssh \
  -t "-o StrictHostKeyChecking=no" \
  -t "-o UserKnownHostsFile=/dev/null" \
  -t "-o PreferredAuthentications=password" \
  -t "-o PubkeyAuthentication=no" \
  -c "chmod +x /home/fedora/gpu-metrics-agent && sudo bash -c 'nohup /home/fedora/gpu-metrics-agent > /tmp/gpu-agent.log 2>&1 &'" \
  fedora@vmi-fedora
```

Verify it's running:

```bash
sshpass -p fedora virtctl ssh \
  -t "-o StrictHostKeyChecking=no" \
  -t "-o UserKnownHostsFile=/dev/null" \
  -t "-o PreferredAuthentications=password" \
  -t "-o PubkeyAuthentication=no" \
  -c "sudo ps aux | grep gpu-metrics-agent" \
  fedora@vmi-fedora
```

Check agent logs:

```bash
sshpass -p fedora virtctl ssh \
  -t "-o StrictHostKeyChecking=no" \
  -t "-o UserKnownHostsFile=/dev/null" \
  -t "-o PreferredAuthentications=password" \
  -t "-o PubkeyAuthentication=no" \
  -c "sudo cat /tmp/gpu-agent.log" \
  fedora@vmi-fedora
```

Without an NVIDIA GPU, the agent will log `NVML init failed` but remain
running and respond to scrape requests with an error payload.

### Windows guest

Run from an elevated command prompt or PowerShell:

```
gpu-metrics-agent.exe
```

The agent opens `\\.\Global\org.kubevirt.gpu-metrics.0` (the virtio-serial
named pipe) and begins serving metrics.

## 5. Verify GPU Metrics in Prometheus

Query the virt-handler metrics endpoint for GPU metrics:

```bash
VIRT_HANDLER=$(kubectl get pods -n kubevirt -l kubevirt.io=virt-handler -o name | head -1)

kubectl exec -n kubevirt ${VIRT_HANDLER} -c virt-handler -- \
  curl -sk https://localhost:8443/metrics | grep "kubevirt_vmi_gpu_"
```

You should see metrics like:

```
kubevirt_vmi_gpu_utilization_percent{gpu_index="0",gpu_name="NVIDIA A100",...} 75
kubevirt_vmi_gpu_memory_utilization_percent{...} 38
kubevirt_vmi_gpu_memory_used_bytes{...} 4294967296
kubevirt_vmi_gpu_memory_total_bytes{...} 1.7179869184e+10
kubevirt_vmi_gpu_temperature_celsius{...} 54
kubevirt_vmi_gpu_power_usage_milliwatts{...} 121180
kubevirt_vmi_gpu_encoder_utilization_percent{...} 15
kubevirt_vmi_gpu_decoder_utilization_percent{...} 5
kubevirt_vmi_gpu_running_processes{...} 3
kubevirt_vmi_gpu_ecc_errors_single_bit_total{...} 0
kubevirt_vmi_gpu_ecc_errors_double_bit_total{...} 0
```

Without an NVIDIA GPU, the agent reports an error response. In that case
no `kubevirt_vmi_gpu_*` metrics will be emitted, but the collection
pipeline (virtio-serial channel, socket, scraper) can still be validated
by checking the agent logs and virt-handler logs for successful communication.

## 6. Cleanup

```bash
kubectl delete vmi vmi-fedora
```

## Agent Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--device` | `/dev/virtio-ports/org.kubevirt.gpu-metrics.0` (Linux) | Virtio-serial device path |
|            | `\\.\Global\org.kubevirt.gpu-metrics.0` (Windows)      |                           |
