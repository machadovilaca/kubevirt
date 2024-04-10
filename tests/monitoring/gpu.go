package monitoring

import (
	"context"

	"k8s.io/apimachinery/pkg/api/resource"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "kubevirt.io/api/core/v1"

	"kubevirt.io/kubevirt/tests"
	"kubevirt.io/kubevirt/tests/decorators"
	"kubevirt.io/kubevirt/tests/framework/kubevirt"
	"kubevirt.io/kubevirt/tests/libvmifact"
)

var _ = Describe("[sig-monitoring]KEKW", Serial, decorators.SigMonitoring, func() {
	Context("GPU resources", func() {
		It("should have GPU resource in virt-launcher", func() {
			vmi := libvmifact.NewGuestless()
			vmi.Spec.Domain.Devices.GPUs = []v1.GPU{
				{
					Name:       "gpu-different-name",
					DeviceName: "nvidia.com/GP102GL_Tesla_P40",
				},
				{
					Name:       "gpu-nvidia-searched-name",
					DeviceName: "nvidia.com/gpu", // https://github.com/NVIDIA/dcgm-exporter/blob/5121ded837c8ddbc5284c8d396ea58a37e1a5480/tests/e2e/internal/framework/kube.go#L31
				},
			}

			_ = tests.RunVMIAndExpectScheduling(vmi, 10)

			virtClient := kubevirt.Client()
			virtLauncherPod, err := virtClient.CoreV1().Pods(vmi.Namespace).List(context.TODO(), metav1.ListOptions{
				LabelSelector: "vm.kubevirt.io/name=" + vmi.Name,
			})
			Expect(err).ToNot(HaveOccurred())

			Expect(virtLauncherPod.Items).To(HaveLen(1))

			requests := virtLauncherPod.Items[0].Spec.Containers[0].Resources.Requests
			Expect(requests["nvidia.com/GP102GL_Tesla_P40"]).To(Equal(resource.MustParse("1")))
			Expect(requests["nvidia.com/gpu"]).To(Equal(resource.MustParse("1")))
		})
	})
})
