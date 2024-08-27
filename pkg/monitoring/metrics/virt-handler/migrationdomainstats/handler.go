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
	"k8s.io/client-go/tools/cache"
	v1 "kubevirt.io/api/core/v1"
)

type Handler struct {
	vmiInformer cache.SharedIndexInformer
	vmiStats    map[string]*queue
}

func NewHandler(vmiInformer cache.SharedIndexInformer) (Handler, error) {
	h := Handler{
		vmiInformer: vmiInformer,
		vmiStats:    make(map[string]*queue),
	}

	_, err := vmiInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: h.handleVmiUpdate,
	})
	if err != nil {
		return Handler{}, err
	}

	return h, nil
}

func (h *Handler) Collect() []Result {
	var allResults []Result

	for _, q := range h.vmiStats {
		vmiResults, isActive := q.all()
		allResults = append(allResults, vmiResults...)

		if !isActive {
			delete(h.vmiStats, namespacedNameKey(q.vmi))
		}
	}

	return allResults
}

func (h *Handler) handleVmiUpdate(_oldObj, newObj interface{}) {
	newVmi := newObj.(*v1.VirtualMachineInstance)

	if newVmi.Status.MigrationState == nil || newVmi.Status.MigrationState.Completed {
		return
	}

	h.addMigration(newVmi)
}

func (h *Handler) addMigration(vmi *v1.VirtualMachineInstance) {
	_, ok := h.vmiStats[namespacedNameKey(vmi)]
	if ok {
		return
	}

	q := newQueue(h.vmiInformer, vmi)
	q.startPolling()
	h.vmiStats[namespacedNameKey(vmi)] = q
}

func namespacedNameKey(vmi *v1.VirtualMachineInstance) string {
	return vmi.Namespace + "/" + vmi.Name
}
