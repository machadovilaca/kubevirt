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

package network

import (
	"errors"
	"fmt"
	"os"

	"kubevirt.io/client-go/log"

	"kubevirt.io/kubevirt/pkg/network/cache"
)

type ConfigStateCache struct {
	vmiUID                string
	cacheCreator          cacheCreator
	volatilePodIfaceState map[string]cache.PodIfaceState
}

func NewConfigStateCache(vmiUID string, cacheCreator cacheCreator) ConfigStateCache {
	return NewConfigStateCacheWithPodIfaceStateData(vmiUID, cacheCreator, map[string]cache.PodIfaceState{})
}

func NewConfigStateCacheWithPodIfaceStateData(vmiUID string, cacheCreator cacheCreator, volatilePodIfaceState map[string]cache.PodIfaceState) ConfigStateCache {
	return ConfigStateCache{vmiUID, cacheCreator, volatilePodIfaceState}
}

func (c *ConfigStateCache) Read(podInterfaceName string) (cache.PodIfaceState, error) {
	if volatilePodIfaceState, ok := c.volatilePodIfaceState[podInterfaceName]; ok {
		return volatilePodIfaceState, nil
	}
	podIfaceCacheData, err := cache.ReadPodInterfaceCache(c.cacheCreator, c.vmiUID, podInterfaceName)
	var state cache.PodIfaceState
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return defaultState, fmt.Errorf("failed to read pod interface network state from cache: %v", err)
		}
		state = defaultState
	} else {
		state = podIfaceCacheData.State
	}
	c.volatilePodIfaceState[podInterfaceName] = state
	return state, nil
}

func (c *ConfigStateCache) Write(podInterfaceName string, state cache.PodIfaceState) error {
	podIfaceCacheData, err := cache.ReadPodInterfaceCache(c.cacheCreator, c.vmiUID, podInterfaceName)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			log.Log.Reason(err).Errorf("failed to read pod interface network (%s) state from cache", podInterfaceName)
			return err
		}
		podIfaceCacheData = &cache.PodIfaceCacheData{}
	}

	podIfaceCacheData.State = state
	err = cache.WritePodInterfaceCache(c.cacheCreator, c.vmiUID, podInterfaceName, podIfaceCacheData)
	if err != nil {
		log.Log.Reason(err).Errorf("failed to write pod interface network (%s) state to cache", podInterfaceName)
		return err
	}
	c.volatilePodIfaceState[podInterfaceName] = state
	return nil
}
