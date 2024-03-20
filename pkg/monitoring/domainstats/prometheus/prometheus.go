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
 * Copyright 2018 Red Hat, Inc.
 *
 */

package prometheus

import (
	"time"

	"k8s.io/client-go/tools/cache"

	vms "kubevirt.io/kubevirt/pkg/monitoring/domainstats"

	"github.com/prometheus/client_golang/prometheus"
	k6tv1 "kubevirt.io/api/core/v1"
	"kubevirt.io/client-go/kubecli"
	"kubevirt.io/client-go/log"

	cmdclient "kubevirt.io/kubevirt/pkg/virt-handler/cmd-client"
	"kubevirt.io/kubevirt/pkg/virt-launcher/virtwrap/stats"
)

const (
	PrometheusCollectionTimeout = vms.CollectionTimeout
)

type DomainStatsCollector struct {
	virtShareDir  string
	nodeName      string
	concCollector *vms.ConcurrentCollector
	vmiInformer   cache.SharedIndexInformer
}

// aggregates to virt-launcher
func SetupDomainStatsCollector(virtCli kubecli.KubevirtClient, virtShareDir, nodeName string, MaxRequestsInFlight int, vmiInformer cache.SharedIndexInformer) *DomainStatsCollector {
	log.Log.Infof("Starting domain stats collector: node name=%v", nodeName)
	co := &DomainStatsCollector{
		virtShareDir:  virtShareDir,
		nodeName:      nodeName,
		concCollector: vms.NewConcurrentCollector(MaxRequestsInFlight),
		vmiInformer:   vmiInformer,
	}

	prometheus.MustRegister(co)
	return co
}

func (co *DomainStatsCollector) Describe(_ chan<- *prometheus.Desc) {
	// TODO: Use DescribeByCollect?
}

// Note that Collect could be called concurrently
func (co *DomainStatsCollector) Collect(ch chan<- prometheus.Metric) {
	cachedObjs := co.vmiInformer.GetIndexer().List()
	if len(cachedObjs) == 0 {
		log.Log.V(4).Infof("No VMIs detected")
		return
	}

	vmis := make([]*k6tv1.VirtualMachineInstance, len(cachedObjs))

	for i, obj := range cachedObjs {
		vmis[i] = obj.(*k6tv1.VirtualMachineInstance)
	}

	scraper := &prometheusScraper{ch: ch}
	co.concCollector.Collect(vmis, scraper, PrometheusCollectionTimeout)
	return
}

type prometheusScraper struct {
	ch chan<- prometheus.Metric
}

type VirtualMachineInstanceStats struct {
	DomainStats *stats.DomainStats
	FsStats     k6tv1.VirtualMachineInstanceFileSystemList
}

func (ps *prometheusScraper) Scrape(socketFile string, vmi *k6tv1.VirtualMachineInstance) {
	ts := time.Now()
	cli, err := cmdclient.NewClient(socketFile)
	if err != nil {
		log.Log.Reason(err).Error("failed to connect to cmd client socket")
		// Ignore failure to connect to client.
		// These are all local connections via unix socket.
		// A failure to connect means there's nothing on the other
		// end listening.
		return
	}
	defer cli.Close()

	vmStats := &VirtualMachineInstanceStats{}
	var exists bool

	vmStats.DomainStats, exists, err = cli.GetDomainStats()
	if err != nil {
		log.Log.Reason(err).Errorf("failed to update domain stats from socket %s", socketFile)
		return
	}
	if !exists || vmStats.DomainStats.Name == "" {
		log.Log.V(2).Infof("disappearing VM on %s, ignored", socketFile) // VM may be shutting down
		return
	}

	vmStats.FsStats, err = cli.GetFilesystems()
	if err != nil {
		log.Log.Reason(err).Errorf("failed to update filesystem stats from socket %s", socketFile)
		return
	}

	// GetDomainStats() may hang for a long time.
	// If it wakes up past the timeout, there is no point in send back any metric.
	// In the best case the information is stale, in the worst case the information is stale *and*
	// the reporting channel is already closed, leading to a possible panic - see below
	elapsed := time.Now().Sub(ts)
	if elapsed > vms.StatsMaxAge {
		log.Log.Infof("took too long (%v) to collect stats from %s: ignored", elapsed, socketFile)
		return
	}

	ps.Report(socketFile, vmi, vmStats)
}

func (ps *prometheusScraper) Report(socketFile string, vmi *k6tv1.VirtualMachineInstance, vmStats *VirtualMachineInstanceStats) {
	// statsMaxAge is an estimation - and there is no better way to do that. So it is possible that
	// GetDomainStats() takes enough time to lag behind, but not enough to trigger the statsMaxAge check.
	// In this case the next functions will end up writing on a closed channel. This will panic.
	// It is actually OK in this case to abort the goroutine that panicked -that's what we want anyway,
	// and the very reason we collect in throwaway goroutines. We need however to avoid dump stacktraces in the logs.
	// Since this is a known failure condition, let's handle it explicitly.
	defer func() {
		if err := recover(); err != nil {
			log.Log.Warningf("collector goroutine panicked for VM %s: %s", socketFile, err)
		}
	}()
}
