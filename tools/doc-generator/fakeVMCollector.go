package main

import (
	"github.com/prometheus/client_golang/prometheus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k6tv1 "kubevirt.io/api/core/v1"

	"kubevirt.io/kubevirt/pkg/monitoring/vmstats"
)

type fakeVMCollector struct {
}

func (fc fakeVMCollector) Describe(_ chan<- *prometheus.Desc) {
}

// Collect needs to report all metrics to see it in docs
func (fc fakeVMCollector) Collect(ch chan<- prometheus.Metric) {
	ps := vmstats.NewPrometheusScraper(ch)

	vms := []*k6tv1.VirtualMachine{
		createVM(k6tv1.VirtualMachineStatusRunning),
	}

	ps.Report(vms)
}

func RegisterFakeVMCollector() {
	prometheus.MustRegister(fakeVMCollector{})
}

func createVM(status k6tv1.VirtualMachinePrintableStatus) *k6tv1.VirtualMachine {
	vmVolumes := []k6tv1.Volume{
		{
			VolumeSource: k6tv1.VolumeSource{
				PersistentVolumeClaim: &k6tv1.PersistentVolumeClaimVolumeSource{
					PersistentVolumeClaimVolumeSource: v1.PersistentVolumeClaimVolumeSource{
						ClaimName: "test-pvc",
					},
				},
			},
		},
	}

	return &k6tv1.VirtualMachine{
		ObjectMeta: metav1.ObjectMeta{Namespace: "test-ns", Name: "test-vm"},
		Spec: k6tv1.VirtualMachineSpec{
			Template: &k6tv1.VirtualMachineInstanceTemplateSpec{
				Spec: k6tv1.VirtualMachineInstanceSpec{
					Volumes: vmVolumes,
				},
			},
		},
		Status: k6tv1.VirtualMachineStatus{
			PrintableStatus: status,
			Conditions: []k6tv1.VirtualMachineCondition{
				{
					Type:               k6tv1.VirtualMachineReady,
					Status:             "any",
					Reason:             "any",
					LastTransitionTime: metav1.Now(),
				},
			},
		},
	}
}
