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
 * Copyright 2022 Red Hat, Inc.
 *
 */

package system

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "kubevirt.io/api/core/v1"

	"kubevirt.io/kubevirt/pkg/virtctl/guestfs"

	k8sv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/prometheus/client_golang/prometheus"
	ioprometheusclient "github.com/prometheus/client_model/go"
)

var _ = Describe("Node Virtualization Status", func() {
	var ch chan prometheus.Metric

	BeforeEach(func() {
		ch = make(chan prometheus.Metric, 5)
	})

	buildNode := func(name string, nodeSchedulable bool, allocatableKvm string) *k8sv1.Node {
		return &k8sv1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
				Labels: map[string]string{
					v1.NodeSchedulable: fmt.Sprint(nodeSchedulable),
				},
			},
			Status: k8sv1.NodeStatus{
				Allocatable: k8sv1.ResourceList{
					guestfs.KvmDevice: resource.MustParse(allocatableKvm),
				},
			},
		}
	}

	It("should set kubevirt_node_virtualization_status metrics to 0 when no node has schedulable label or virtualization extensions", func() {
		nodes := []*k8sv1.Node{
			buildNode("node1", false, "0"),
			buildNode("node2", true, "0"),
			buildNode("node2", false, "1k"),
		}
		collectNodesVirtualizationStatus(ch, nodes)

		close(ch)
		Expect(ch).To(HaveLen(len(nodes)))
		for m := range ch {
			Expect(m.Desc().String()).To(ContainSubstring("kubevirt_node_virtualization_status"))
			dto := &ioprometheusclient.Metric{}
			m.Write(dto)
			Expect(*dto.Gauge.Value).Should(BeZero())
		}
	})

	It("should set kubevirt_node_virtualization_status metrics to 1 when node has schedulable label and virtualization extensions", func() {
		nodes := []*k8sv1.Node{
			buildNode("node1", true, "1k"),
		}
		collectNodesVirtualizationStatus(ch, nodes)

		close(ch)
		Expect(ch).To(HaveLen(len(nodes)))
		for m := range ch {
			Expect(m.Desc().String()).To(ContainSubstring("kubevirt_node_virtualization_status"))
			dto := &ioprometheusclient.Metric{}
			m.Write(dto)
			Expect(*dto.Gauge.Value).Should(Equal(1.0))
		}
	})
})
