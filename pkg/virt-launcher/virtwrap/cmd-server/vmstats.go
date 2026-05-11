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
 * Copyright The KubeVirt Authors.
 *
 */

package cmdserver

import (
	"context"
	"fmt"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/util/json"

	"kubevirt.io/client-go/log"

	cmdv1 "kubevirt.io/kubevirt/pkg/handler-launcher-com/cmd/v1"
)

func (l *Launcher) GetVMStats(_ context.Context, request *cmdv1.VMStatsRequest) (*cmdv1.VMStatsResponse, error) {
	response := &cmdv1.VMStatsResponse{
		Response: &cmdv1.Response{
			Success: true,
		},
	}

	var errs []string

	if request.DomainStats != nil {
		domainStats, err := l.domainManager.GetDomainStats()
		if err != nil {
			log.Log.Reason(err).Error("Failed to get domain stats")
			errs = append(errs, fmt.Sprintf("domain stats: %s", getErrorMessage(err)))
		} else {
			data, err := json.Marshal(domainStats)
			if err != nil {
				log.Log.Reason(err).Error("Failed to marshal domain stats")
				errs = append(errs, fmt.Sprintf("marshal domain stats: %s", getErrorMessage(err)))
			} else {
				response.DomainStats = string(data)
			}
		}
	}

	if request.DirtyRate != nil {
		const dirtyRateCalculationTime = time.Second
		dirtyRate, err := l.domainManager.GetDomainDirtyRateStats(dirtyRateCalculationTime)
		if err != nil {
			log.Log.Reason(err).Error("Failed to get domain dirty rate stats")
			errs = append(errs, fmt.Sprintf("dirty rate stats: %s", getErrorMessage(err)))
		} else {
			data, err := json.Marshal(dirtyRate)
			if err != nil {
				log.Log.Reason(err).Error("Failed to marshal dirty rate stats")
				errs = append(errs, fmt.Sprintf("marshal dirty rate stats: %s", getErrorMessage(err)))
			} else {
				response.DirtyRateStats = string(data)
			}
		}
	}

	if len(request.AgentData) > 0 {
		response.GuestAgentVersion = l.domainManager.GetGuestAgentVersion()

		response.AgentData = make(map[string]string, len(request.AgentData))
		for _, req := range request.AgentData {
			if data, ok := l.domainManager.GetAgentData(req.CommandKey); ok {
				response.AgentData[req.CommandKey] = data
			}
		}
	}

	if len(errs) > 0 {
		response.Response.Success = false
		response.Response.Message = strings.Join(errs, "; ")
	}

	return response, nil
}
