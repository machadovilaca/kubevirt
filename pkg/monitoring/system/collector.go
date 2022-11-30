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
	"github.com/prometheus/client_golang/prometheus"
	k8sv1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/cache"

	"kubevirt.io/client-go/log"
)

type Collector struct {
	nodeInformer cache.SharedIndexInformer
}

func (co *Collector) Describe(_ chan<- *prometheus.Desc) {
	// TODO: Use DescribeByCollect?
}

func SetupCollector(nodeInformer cache.SharedIndexInformer) *Collector {
	log.Log.Infof("Starting system collector")
	co := &Collector{
		nodeInformer: nodeInformer,
	}

	prometheus.MustRegister(co)
	return co
}

func (co *Collector) Collect(ch chan<- prometheus.Metric) {
	log.Log.Infof("Collecting system metrics")

	cachedObjs := co.nodeInformer.GetIndexer().List()
	if len(cachedObjs) == 0 {
		log.Log.Infof("No nodes found")
		return
	}

	nodes := make([]*k8sv1.Node, len(cachedObjs))
	for i, obj := range cachedObjs {
		nodes[i] = obj.(*k8sv1.Node)
	}

	for _, node := range nodes {
		log.Log.Infof(node.Name)
	}
}
