/*
 * This file is part of the KubeVirt project
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 * Copyright 2023 Red Hat, Inc.
 *
 */

package monitoring

import (
	"context"
	"fmt"
	"strconv"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"

	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"

	v1 "kubevirt.io/api/core/v1"
	"kubevirt.io/client-go/kubecli"

	virtctlpause "kubevirt.io/kubevirt/pkg/virtctl/pause"

	"kubevirt.io/kubevirt/tests"
	"kubevirt.io/kubevirt/tests/clientcmd"
	cd "kubevirt.io/kubevirt/tests/containerdisk"
	"kubevirt.io/kubevirt/tests/decorators"
	"kubevirt.io/kubevirt/tests/framework/checks"
	"kubevirt.io/kubevirt/tests/framework/kubevirt"
	"kubevirt.io/kubevirt/tests/framework/matcher"
	"kubevirt.io/kubevirt/tests/libnode"
	"kubevirt.io/kubevirt/tests/libvmi"
	"kubevirt.io/kubevirt/tests/libwait"
	"kubevirt.io/kubevirt/tests/testsuite"
	"kubevirt.io/kubevirt/tests/util"
)

var _ = Describe("[Serial][sig-monitoring]VM Monitoring", Serial, decorators.SigMonitoring, func() {
	var err error
	var virtClient kubecli.KubevirtClient

	BeforeEach(func() {
		virtClient = kubevirt.Client()
	})

	Context("VM status metrics", func() {
		var vm *v1.VirtualMachine
		var cpuMetrics = []string{
			"kubevirt_vmi_cpu_system_usage_seconds",
			"kubevirt_vmi_cpu_usage_seconds",
			"kubevirt_vmi_cpu_user_usage_seconds",
		}

		BeforeEach(func() {
			vmi := tests.NewRandomVMI()
			vm = tests.NewRandomVirtualMachine(vmi, false)

			By("Create a VirtualMachine")
			_, err = virtClient.VirtualMachine(vm.Namespace).Create(context.Background(), vm)
			Expect(err).ToNot(HaveOccurred())
		})

		checkMetricTo := func(metric string, labels map[string]string, matcher types.GomegaMatcher, description string) {
			EventuallyWithOffset(1, func() int {
				v, err := getMetricValueWithLabels(virtClient, metric, labels)
				if err != nil {
					return -1
				}
				i, err := strconv.Atoi(v)
				Expect(err).ToNot(HaveOccurred())
				return i
			}, 3*time.Minute, 20*time.Second).Should(matcher, description)
		}

		It("Should be available for a running VM", func() {
			By("Start the VM")
			vm = tests.StartVirtualMachine(vm)

			By("Checking that the VM metrics are available")
			metricLabels := map[string]string{"name": vm.Name, "namespace": vm.Namespace}
			for _, metric := range cpuMetrics {
				checkMetricTo(metric, metricLabels, BeNumerically(">=", 0), "VM metrics should be available for a running VM")
			}
		})

		It("Should be available for a paused VM", func() {
			By("Start the VM")
			vm = tests.StartVirtualMachine(vm)

			By("Pausing the VM")
			command := clientcmd.NewRepeatableVirtctlCommand(virtctlpause.COMMAND_PAUSE, "vm", "--namespace", testsuite.GetTestNamespace(vm), vm.Name)
			Expect(command()).To(Succeed())

			By("Waiting until next Prometheus scrape")
			time.Sleep(35 * time.Second)

			By("Checking that the VM metrics are available")
			metricLabels := map[string]string{"name": vm.Name, "namespace": vm.Namespace}
			for _, metric := range cpuMetrics {
				checkMetricTo(metric, metricLabels, BeNumerically(">=", 0), "VM metrics should be available for a paused VM")
			}
		})

		It("Should not be available for a stopped VM", func() {
			By("Checking that the VM metrics are not available")
			metricLabels := map[string]string{"name": vm.Name, "namespace": vm.Namespace}
			for _, metric := range cpuMetrics {
				checkMetricTo(metric, metricLabels, BeNumerically("==", -1), "VM metrics should not be available for a stopped VM")
			}
		})
	})

	Context("VM migration metrics", func() {
		var nodes *corev1.NodeList

		BeforeEach(func() {
			checks.SkipIfMigrationIsNotPossible()

			Eventually(func() []corev1.Node {
				nodes = libnode.GetAllSchedulableNodes(virtClient)
				return nodes.Items
			}, 60*time.Second, 1*time.Second).ShouldNot(BeEmpty(), "There should be some compute node")
		})

		It("Should correctly update metrics on successful VMIM", func() {
			By("Creating VMIs")
			vmi := tests.NewRandomFedoraVMIWithGuestAgent()
			vmi = tests.RunVMIAndExpectLaunch(vmi, 240)

			By("Migrating VMIs")
			migration := tests.NewRandomMigration(vmi.Name, vmi.Namespace)
			tests.RunMigrationAndExpectCompletion(virtClient, migration, tests.MigrationWaitTime)

			waitForMetricValue(virtClient, "kubevirt_migrate_vmi_pending_count", 0)
			waitForMetricValue(virtClient, "kubevirt_migrate_vmi_scheduling_count", 0)
			waitForMetricValue(virtClient, "kubevirt_migrate_vmi_running_count", 0)

			labels := map[string]string{
				"vmi": vmi.Name,
			}
			waitForMetricValueWithLabels(virtClient, "kubevirt_migrate_vmi_succeeded", 1, labels)

			By("Delete VMIs")
			Expect(virtClient.VirtualMachineInstance(vmi.Namespace).Delete(context.Background(), vmi.Name, &metav1.DeleteOptions{})).To(Succeed())
			libwait.WaitForVirtualMachineToDisappearWithTimeout(vmi, 240)
		})

		It("Should correctly update metrics on failing VMIM", func() {
			By("Creating VMIs")
			vmi := libvmi.NewFedora(
				libvmi.WithInterface(libvmi.InterfaceDeviceWithMasqueradeBinding()),
				libvmi.WithNetwork(v1.DefaultPodNetwork()),
				libvmi.WithNodeAffinityFor(&nodes.Items[0]),
			)
			vmi = tests.RunVMIAndExpectLaunch(vmi, 240)
			labels := map[string]string{
				"vmi": vmi.Name,
			}

			By("Starting the Migration")
			migration := tests.NewRandomMigration(vmi.Name, vmi.Namespace)
			migration.Annotations = map[string]string{v1.MigrationUnschedulablePodTimeoutSecondsAnnotation: "60"}
			migration = tests.RunMigration(virtClient, migration)

			waitForMetricValue(virtClient, "kubevirt_migrate_vmi_scheduling_count", 1)

			Eventually(matcher.ThisMigration(migration), 2*time.Minute, 5*time.Second).Should(matcher.BeInPhase(v1.MigrationFailed), "migration creation should fail")

			waitForMetricValue(virtClient, "kubevirt_migrate_vmi_scheduling_count", 0)
			waitForMetricValueWithLabels(virtClient, "kubevirt_migrate_vmi_failed", 1, labels)

			By("Deleting the VMI")
			Expect(virtClient.VirtualMachineInstance(vmi.Namespace).Delete(context.Background(), vmi.Name, &metav1.DeleteOptions{})).To(Succeed())
			libwait.WaitForVirtualMachineToDisappearWithTimeout(vmi, 240)
		})
	})

	Context("VM snapshot metrics", func() {
		quantity, _ := resource.ParseQuantity("500Mi")

		createSimplePVCWithRestoreLabels := func(name string) {
			_, err := virtClient.CoreV1().PersistentVolumeClaims(util.NamespaceTestDefault).Create(context.Background(), &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name: name,
					Labels: map[string]string{
						"restore.kubevirt.io/source-vm-name":      "simple-vm",
						"restore.kubevirt.io/source-vm-namespace": util.NamespaceTestDefault,
					},
				},
				Spec: corev1.PersistentVolumeClaimSpec{
					AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							"storage": quantity,
						},
					},
				},
			}, metav1.CreateOptions{})
			Expect(err).ToNot(HaveOccurred())
		}

		It("[test_id:8639]Number of disks restored and total restored bytes metric values should be correct", func() {
			totalMetric := fmt.Sprintf("kubevirt_vmsnapshot_disks_restored_from_source_total{vm_name='simple-vm',vm_namespace='%s'}", util.NamespaceTestDefault)
			bytesMetric := fmt.Sprintf("kubevirt_vmsnapshot_disks_restored_from_source_bytes{vm_name='simple-vm',vm_namespace='%s'}", util.NamespaceTestDefault)
			numPVCs := 2

			for i := 1; i < numPVCs+1; i++ {
				// Create dummy PVC that is labelled as "restored" from VM snapshot
				createSimplePVCWithRestoreLabels(fmt.Sprintf("vmsnapshot-restored-pvc-%d", i))
				// Metric values increases per restored disk
				waitForMetricValue(virtClient, totalMetric, int64(i))
				waitForMetricValue(virtClient, bytesMetric, quantity.Value()*int64(i))
			}
		})
	})

	Context("VM alerts", func() {
		createSC := func(name string) {
			_, err := virtClient.StorageV1().StorageClasses().Get(context.Background(), name, metav1.GetOptions{})
			if err != nil {
				Expect(errors.IsNotFound(err)).To(BeTrue())
				sc := &storagev1.StorageClass{
					ObjectMeta: metav1.ObjectMeta{
						Name: "ocs-storagecluster-ceph-rbd",
					},
					Provisioner: "openshift-storage.rbd.csi.ceph.com",
				}
				_, err = virtClient.StorageV1().StorageClasses().Create(context.Background(), sc, metav1.CreateOptions{})
				Expect(err).ToNot(HaveOccurred())
			}
		}

		createPVC := func(storageClass string, name string) {
			pvc := &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name: name,
				},
				Spec: corev1.PersistentVolumeClaimSpec{
					StorageClassName: &storageClass,
					AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							"storage": resource.MustParse("1Gi"),
						},
					},
				},
			}
			_, err := virtClient.CoreV1().PersistentVolumeClaims(util.NamespaceTestDefault).Create(context.Background(), pvc, metav1.CreateOptions{})
			Expect(err).ToNot(HaveOccurred())
		}

		createVM := func(name string) {
			vmWin10 := tests.NewRandomVMWithEphemeralDisk(cd.ContainerDiskFor(cd.ContainerDiskAlpine))
			vmWin10.Spec.Template.ObjectMeta.Annotations = map[string]string{
				"vm.kubevirt.io/os": "windows10",
			}
			vmWin10.Spec.Template.Spec.Volumes = append(vmWin10.Spec.Template.Spec.Volumes, v1.Volume{
				Name: "windows-vm",
				VolumeSource: v1.VolumeSource{
					PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
						PersistentVolumeClaimVolumeSource: corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: name,
						},
					},
				},
			})
			_, err := virtClient.VirtualMachine(vmWin10.Namespace).Create(context.Background(), vmWin10)
			Expect(err).ToNot(HaveOccurred())
		}

		It("should throw WindowsVirtualMachineMayReportOSDErrors when a windows VMs is using default ODF storage", func() {
			defaultODFSCName := "ocs-storagecluster-ceph-rbd"
			createSC(defaultODFSCName)

			randVMName := "windows-vm-" + rand.String(5)
			createPVC(defaultODFSCName, randVMName)
			createVM(randVMName)

			verifyAlertExist(virtClient, "WindowsVirtualMachineMayReportOSDErrors")
		})
	})
})
