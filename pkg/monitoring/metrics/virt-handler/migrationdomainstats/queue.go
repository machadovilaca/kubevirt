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
 * Copyright the KubeVirt Authors.
 */

package migrationdomainstats

import (
	"fmt"
	"sync"
	"time"

	"k8s.io/client-go/tools/cache"

	v1 "kubevirt.io/api/core/v1"
	"kubevirt.io/client-go/log"

	"kubevirt.io/kubevirt/pkg/monitoring/metrics/virt-handler/domainstats"
	domstatsCollector "kubevirt.io/kubevirt/pkg/monitoring/metrics/virt-handler/domainstats/collector"
	"kubevirt.io/kubevirt/pkg/virt-launcher/virtwrap/stats"
)

const (
	collectionTimeout = 10 * time.Second
	pollingInterval   = 5 * time.Second
)

type Result struct {
	VMI       string
	Namespace string

	DomainJobInfo stats.DomainJobInfo
	Timestamp     time.Time
}

type queue struct {
	vmiInformer cache.SharedIndexInformer

	vmi     *v1.VirtualMachineInstance
	results []Result

	isActive  bool
	mutex     *sync.Mutex
	collector domstatsCollector.Collector
}

func newQueue(vmiInformer cache.SharedIndexInformer, vmi *v1.VirtualMachineInstance) *queue {
	return &queue{
		vmiInformer: vmiInformer,

		vmi:     vmi,
		results: make([]Result, 0),

		isActive:  false,
		mutex:     &sync.Mutex{},
		collector: domstatsCollector.NewConcurrentCollector(1),
	}
}

func (q *queue) startPolling() {
	q.isActive = true

	ticker := time.NewTicker(pollingInterval)
	go func() {
		for range ticker.C {
			if !q.isActive {
				ticker.Stop()
				return
			}
			q.collect()
		}
	}()
}

func (q *queue) collect() {
	if q.isMigrationFinished() {
		q.isActive = false
		return
	}

	values, err := q.scrapeDomainStats()
	if err != nil {
		log.Log.Reason(err).Errorf("failed to scrape domain stats for VMI %s/%s", q.vmi.Namespace, q.vmi.Name)
		return
	}

	result := Result{
		VMI:       q.vmi.Name,
		Namespace: q.vmi.Namespace,

		DomainJobInfo: *values.MigrateDomainJobInfo,
		Timestamp:     time.Now(),
	}

	q.mutex.Lock()
	defer q.mutex.Unlock()
	q.results = append(q.results, result)
}

func (q *queue) scrapeDomainStats() (*stats.DomainStats, error) {
	scraper := domainstats.NewDomainstatsScraper(1)
	vmis := []*v1.VirtualMachineInstance{q.vmi}
	q.collector.Collect(vmis, scraper, collectionTimeout)

	values := scraper.GetValues()
	if len(values) != 1 {
		return nil, fmt.Errorf("expected 1 value from DomainstatsScraper, got %d", len(values))
	}

	return values[0].GetVmiStats().DomainStats, nil
}

func (q *queue) all() ([]Result, bool) {
	q.mutex.Lock()
	defer q.mutex.Unlock()

	out := q.results
	q.results = make([]Result, 0)

	return out, q.isActive
}

func (q *queue) isMigrationFinished() bool {
	vmiRaw, exists, err := q.vmiInformer.GetStore().Get(q.vmi)
	if err != nil {
		log.Log.Reason(err).Errorf("failed to get VMI %s/%s", q.vmi.Namespace, q.vmi.Name)
		return true
	}
	if !exists {
		return true
	}

	vmi := vmiRaw.(*v1.VirtualMachineInstance)
	if vmi.Status.MigrationState == nil || vmi.Status.MigrationState.Completed {
		return true
	}

	return false
}
