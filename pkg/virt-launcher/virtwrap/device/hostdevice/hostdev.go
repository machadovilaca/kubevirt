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
 * Copyright 2021 Red Hat, Inc.
 *
 */

package hostdevice

import (
	"fmt"

	"kubevirt.io/client-go/log"
	"kubevirt.io/kubevirt/pkg/virt-launcher/virtwrap/api"
	"kubevirt.io/kubevirt/pkg/virt-launcher/virtwrap/device"
)

type HostDeviceMetaData struct {
	AliasPrefix  string
	Name         string
	ResourceName string

	// DecorateHook is a function pointer that may be used to mutate the domain host-device
	// with additional specific parameters. E.g. guest PCI address.
	DecorateHook func(hostDevice *api.HostDevice) error
}

type createHostDevice func(HostDeviceMetaData, string) (*api.HostDevice, error)

type AddressPooler interface {
	Pop(key string) (value string, err error)
}

func CreatePCIHostDevices(hostDevicesData []HostDeviceMetaData, pciAddrPool AddressPooler) ([]api.HostDevice, error) {
	return createHostDevices(hostDevicesData, pciAddrPool, createPCIHostDevice)
}

func CreateMDEVHostDevices(hostDevicesData []HostDeviceMetaData, mdevAddrPool AddressPooler) ([]api.HostDevice, error) {
	return createHostDevices(hostDevicesData, mdevAddrPool, createMDEVHostDevice)
}

func createHostDevices(hostDevicesData []HostDeviceMetaData, addrPool AddressPooler, createHostDev createHostDevice) ([]api.HostDevice, error) {
	var hostDevices []api.HostDevice

	for _, hostDeviceData := range hostDevicesData {
		address, err := addrPool.Pop(hostDeviceData.ResourceName)
		if err != nil {
			return nil, fmt.Errorf("failed to create hostdevice for %s: %v", hostDeviceData.Name, err)
		}

		hostDevice, err := createHostDev(hostDeviceData, address)
		if err != nil {
			return nil, fmt.Errorf("failed to create hostdevice for %s: %v", hostDeviceData.Name, err)
		}
		if hostDeviceData.DecorateHook != nil {
			if err := hostDeviceData.DecorateHook(hostDevice); err != nil {
				return nil, fmt.Errorf("failed to create hostdevice for %s: %v", hostDeviceData.Name, err)
			}
		}
		hostDevices = append(hostDevices, *hostDevice)
		log.Log.Infof("host-device created: %s", address)
	}
	return hostDevices, nil
}

func createPCIHostDevice(hostDeviceData HostDeviceMetaData, hostPCIAddress string) (*api.HostDevice, error) {
	hostAddr, err := device.NewPciAddressField(hostPCIAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to create PCI device for %s: %v", hostDeviceData.Name, err)
	}
	domainHostDevice := &api.HostDevice{
		Alias:   api.NewUserDefinedAlias(hostDeviceData.AliasPrefix + hostDeviceData.Name),
		Source:  api.HostDeviceSource{Address: hostAddr},
		Type:    "pci",
		Managed: "no",
	}
	return domainHostDevice, nil
}

func createMDEVHostDevice(hostDeviceData HostDeviceMetaData, mdevUUID string) (*api.HostDevice, error) {
	domainHostDevice := &api.HostDevice{
		Alias: api.NewUserDefinedAlias(hostDeviceData.AliasPrefix + hostDeviceData.Name),
		Source: api.HostDeviceSource{
			Address: &api.Address{
				UUID: mdevUUID,
			},
		},
		Type:  "mdev",
		Mode:  "subsystem",
		Model: "vfio-pci",
	}
	return domainHostDevice, nil
}
