/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
// copied from kubernetes/pkg/kubelet/cm/cpumanager/cpu_assignment_test.go

package cm

import (
	"reflect"
	"sort"
	"testing"

	"k8s.io/kubernetes/pkg/kubelet/cm/cpumanager/topology"
	"k8s.io/kubernetes/pkg/kubelet/cm/cpuset"
)

var (
	topoSingleSocketHT = &CPUTopology{
		&topology.CPUTopology{
			NumCPUs:    8,
			NumSockets: 1,
			NumCores:   4,
			CPUDetails: map[int]topology.CPUInfo{
				0: {CoreID: 0, SocketID: 0, NUMANodeID: 0},
				1: {CoreID: 1, SocketID: 0, NUMANodeID: 0},
				2: {CoreID: 2, SocketID: 0, NUMANodeID: 0},
				3: {CoreID: 3, SocketID: 0, NUMANodeID: 0},
				4: {CoreID: 0, SocketID: 0, NUMANodeID: 0},
				5: {CoreID: 1, SocketID: 0, NUMANodeID: 0},
				6: {CoreID: 2, SocketID: 0, NUMANodeID: 0},
				7: {CoreID: 3, SocketID: 0, NUMANodeID: 0},
			},
		},
		&CCDTopology{
			NumCCDs:       2,
			CPUCCDDetails: map[int]CPUInfo{
				0: {CCDID: 0},
				1: {CCDID: 1},
				2: {CCDID: 0},
				3: {CCDID: 1},
				4: {CCDID: 0},
				5: {CCDID: 1},
				6: {CCDID: 0},
				7: {CCDID: 1},
			},
		},
	}

	topoDualSocketHT = &CPUTopology{
		&topology.CPUTopology{
			NumCPUs:    16,
			NumSockets: 2,
			NumCores:   8,
			CPUDetails: map[int]topology.CPUInfo{
				0:  {CoreID: 0, SocketID: 0, NUMANodeID: 0},
				1:  {CoreID: 1, SocketID: 1, NUMANodeID: 1},
				2:  {CoreID: 2, SocketID: 0, NUMANodeID: 0},
				3:  {CoreID: 3, SocketID: 1, NUMANodeID: 1},
				4:  {CoreID: 4, SocketID: 0, NUMANodeID: 0},
				5:  {CoreID: 5, SocketID: 1, NUMANodeID: 1},
				6:  {CoreID: 6, SocketID: 0, NUMANodeID: 0},
				7:  {CoreID: 7, SocketID: 1, NUMANodeID: 1},
				8:  {CoreID: 0, SocketID: 0, NUMANodeID: 0},
				9:  {CoreID: 1, SocketID: 1, NUMANodeID: 1},
				10: {CoreID: 2, SocketID: 0, NUMANodeID: 0},
				11: {CoreID: 3, SocketID: 1, NUMANodeID: 1},
				12: {CoreID: 4, SocketID: 0, NUMANodeID: 0},
				13: {CoreID: 5, SocketID: 1, NUMANodeID: 1},
				14: {CoreID: 6, SocketID: 0, NUMANodeID: 0},
				15: {CoreID: 7, SocketID: 1, NUMANodeID: 1},
			},
		},
		&CCDTopology{
			NumCCDs:       4,
			CPUCCDDetails: map[int]CPUInfo{
				0: {CCDID: 0},
				1: {CCDID: 1},
				2: {CCDID: 2},
				3: {CCDID: 3},
				4: {CCDID: 0},
				5: {CCDID: 1},
				6: {CCDID: 2},
				7: {CCDID: 3},
				8: {CCDID: 0},
				9: {CCDID: 1},
				10: {CCDID: 2},
				11: {CCDID: 3},
				12: {CCDID: 0},
				13: {CCDID: 1},
				14: {CCDID: 2},
				15: {CCDID: 3},
			},
		},
	}
)

func TestCPUAccumulatorFreeSockets(t *testing.T) {
	testCases := []struct {
		description   string
		topo          *CPUTopology
		availableCPUs cpuset.CPUSet
		expect        []int
	}{
		{
			"single socket HT, 1 socket free",
			topoSingleSocketHT,
			cpuset.NewCPUSet(0, 1, 2, 3, 4, 5, 6, 7),
			[]int{0},
		},
		{
			"single socket HT, 0 sockets free",
			topoSingleSocketHT,
			cpuset.NewCPUSet(1, 2, 3, 4, 5, 6, 7),
			[]int{},
		},
		{
			"dual socket HT, 2 sockets free",
			topoDualSocketHT,
			cpuset.NewCPUSet(0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15),
			[]int{0, 1},
		},
		{
			"dual socket HT, 1 socket free",
			topoDualSocketHT,
			cpuset.NewCPUSet(0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 11, 12, 13, 14, 15),
			[]int{1},
		},
		{
			"dual socket HT, 0 sockets free",
			topoDualSocketHT,
			cpuset.NewCPUSet(0, 2, 3, 4, 5, 6, 7, 8, 9, 11, 12, 13, 14, 15),
			[]int{},
		},
	}

	for _, tc := range testCases {
		acc := newCPUAccumulator(tc.topo, tc.availableCPUs, 0)
		result := acc.freeSockets()
		if !reflect.DeepEqual(result, tc.expect) {
			t.Errorf("[%s] expected %v to equal %v", tc.description, result, tc.expect)
		}
	}
}

func TestCPUAccumulatorFreeCCDs(t *testing.T) {
	testCases := []struct {
		description   string
		topo          *CPUTopology
		availableCPUs cpuset.CPUSet
		expect        []int
	}{
		{
			"single socket HT, 1 ccd free",
			topoSingleSocketHT,
			cpuset.NewCPUSet(0, 1, 2, 3, 4, 5, 6),
			[]int{0},
		},
		{
			"single socket HT, 0 ccd free",
			topoSingleSocketHT,
			cpuset.NewCPUSet(1, 2, 3, 4, 5, 6),
			[]int{},
		},
		{
			"dual socket HT, 4 ccd free",
			topoDualSocketHT,
			cpuset.NewCPUSet(0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15),
			[]int{0, 2, 1, 3},
		},
		{
			"dual socket HT, 1 ccd free",
			topoDualSocketHT,
			cpuset.NewCPUSet(1, 2, 4, 5, 6, 7, 8, 9, 11, 12, 13, 14, 15),
			[]int{1},
		},
		{
			"dual socket HT, 0 ccd free",
			topoDualSocketHT,
			cpuset.NewCPUSet(0, 2, 3, 4, 5, 6, 7, 8, 9, 11),
			[]int{},
		},
	}

	for _, tc := range testCases {
		acc := newCPUAccumulator(tc.topo, tc.availableCPUs, 0)
		result := acc.freeCCDs()
		if !reflect.DeepEqual(result, tc.expect) {
			t.Errorf("[%s] expected %v to equal %v", tc.description, result, tc.expect)
		}
	}
}

func TestCPUAccumulatorFreeCores(t *testing.T) {
	testCases := []struct {
		description   string
		topo          *CPUTopology
		availableCPUs cpuset.CPUSet
		expect        []int
	}{
		{
			"single socket HT, 4 cores free",
			topoSingleSocketHT,
			cpuset.NewCPUSet(0, 1, 2, 3, 4, 5, 6, 7),
			[]int{0, 2, 1, 3},
		},
		{
			"single socket HT, 3 cores free",
			topoSingleSocketHT,
			cpuset.NewCPUSet(0, 1, 2, 4, 5, 6),
			[]int{1, 0, 2},
		},
		{
			"single socket HT, 3 cores free (1 partially consumed)",
			topoSingleSocketHT,
			cpuset.NewCPUSet(0, 1, 2, 3, 4, 5, 6),
			[]int{1, 0, 2},
		},
		{
			"single socket HT, 0 cores free",
			topoSingleSocketHT,
			cpuset.NewCPUSet(),
			[]int{},
		},
		{
			"single socket HT, 0 cores free (4 partially consumed)",
			topoSingleSocketHT,
			cpuset.NewCPUSet(0, 1, 2, 3),
			[]int{},
		},
		{
			"dual socket HT, 6 cores free",
			topoDualSocketHT,
			cpuset.NewCPUSet(0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15),
			[]int{0, 4, 2, 6, 1, 5, 3, 7},
		},
		{
			"dual socket HT, 5 cores free (1 consumed from socket 0)",
			topoDualSocketHT,
			cpuset.NewCPUSet(2, 1, 3, 4, 5, 7, 8, 9, 10, 11, 12, 13, 14, 15),
			[]int{4, 2, 1, 5, 3, 7},
		},
		{
			"dual socket HT, 4 cores free (1 consumed from each socket)",
			topoDualSocketHT,
			cpuset.NewCPUSet(2, 3, 4, 5, 8, 9, 10, 11, 12, 13, 14, 15),
			[]int{4, 2, 5, 3},
		},
	}

	for _, tc := range testCases {
		acc := newCPUAccumulator(tc.topo, tc.availableCPUs, 0)
		result := acc.freeCores()
		if !reflect.DeepEqual(result, tc.expect) {
			t.Errorf("[%s] expected %v to equal %v", tc.description, result, tc.expect)
		}
	}
}

func TestCPUAccumulatorFreeCPUs(t *testing.T) {
	testCases := []struct {
		description   string
		topo          *CPUTopology
		availableCPUs cpuset.CPUSet
		expect        []int
	}{
		{
			"single socket HT, 8 cpus free",
			topoSingleSocketHT,
			cpuset.NewCPUSet(0, 1, 2, 3, 4, 5, 6, 7),
			[]int{0, 4, 2, 6, 1, 5, 3, 7},
		},
		{
			"single socket HT, 5 cpus free",
			topoSingleSocketHT,
			cpuset.NewCPUSet(3, 4, 5, 6, 7),
			[]int{4, 6, 5, 3, 7},
		},
		{
			"dual socket HT, 16 cpus free",
			topoDualSocketHT,
			cpuset.NewCPUSet(0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15),
			[]int{0, 8, 4, 12, 2, 10, 6, 14, 1, 9, 5, 13, 3, 11, 7, 15},
		},
		{
			"dual socket HT, 11 cpus free",
			topoDualSocketHT,
			cpuset.NewCPUSet(1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11),
			[]int{8, 1, 9, 4, 6, 5, 2, 10, 3, 11, 7},
		},
		{
			"dual socket HT, 10 cpus free",
			topoDualSocketHT,
			cpuset.NewCPUSet(1, 2, 3, 4, 5, 7, 8, 9, 10, 11),
			[]int{8, 4, 5, 7, 2, 10, 1, 9, 3, 11},
		},
	}

	for _, tc := range testCases {
		acc := newCPUAccumulator(tc.topo, tc.availableCPUs, 0)
		result := acc.freeCPUs()
		if !reflect.DeepEqual(result, tc.expect) {
			t.Errorf("[%s] expected %v to equal %v", tc.description, result, tc.expect)
		}
	}
}

func TestCPUAccumulatorTake(t *testing.T) {
	testCases := []struct {
		description     string
		topo            *CPUTopology
		availableCPUs   cpuset.CPUSet
		takeCPUs        []cpuset.CPUSet
		numCPUs         int
		expectSatisfied bool
		expectFailed    bool
	}{
		{
			"take 0 cpus from a single socket HT, require 1",
			topoSingleSocketHT,
			cpuset.NewCPUSet(0, 1, 2, 3, 4, 5, 6, 7),
			[]cpuset.CPUSet{cpuset.NewCPUSet()},
			1,
			false,
			false,
		},
		{
			"take 0 cpus from a single socket HT, require 1, none available",
			topoSingleSocketHT,
			cpuset.NewCPUSet(),
			[]cpuset.CPUSet{cpuset.NewCPUSet()},
			1,
			false,
			true,
		},
		{
			"take 1 cpu from a single socket HT, require 1",
			topoSingleSocketHT,
			cpuset.NewCPUSet(0, 1, 2, 3, 4, 5, 6, 7),
			[]cpuset.CPUSet{cpuset.NewCPUSet(0)},
			1,
			true,
			false,
		},
		{
			"take 1 cpu from a single socket HT, require 2",
			topoSingleSocketHT,
			cpuset.NewCPUSet(0, 1, 2, 3, 4, 5, 6, 7),
			[]cpuset.CPUSet{cpuset.NewCPUSet(0)},
			2,
			false,
			false,
		},
		{
			"take 2 cpu from a single socket HT, require 4, expect failed",
			topoSingleSocketHT,
			cpuset.NewCPUSet(0, 1, 2),
			[]cpuset.CPUSet{cpuset.NewCPUSet(0), cpuset.NewCPUSet(1)},
			4,
			false,
			true,
		},
		{
			"take all cpus one at a time from a single socket HT, require 8",
			topoSingleSocketHT,
			cpuset.NewCPUSet(0, 1, 2, 3, 4, 5, 6, 7),
			[]cpuset.CPUSet{
				cpuset.NewCPUSet(0),
				cpuset.NewCPUSet(1),
				cpuset.NewCPUSet(2),
				cpuset.NewCPUSet(3),
				cpuset.NewCPUSet(4),
				cpuset.NewCPUSet(5),
				cpuset.NewCPUSet(6),
				cpuset.NewCPUSet(7),
			},
			8,
			true,
			false,
		},
	}

	for _, tc := range testCases {
		acc := newCPUAccumulator(tc.topo, tc.availableCPUs, tc.numCPUs)
		totalTaken := 0
		for _, cpus := range tc.takeCPUs {
			acc.take(cpus)
			totalTaken += cpus.Size()
		}
		if tc.expectSatisfied != acc.isSatisfied() {
			t.Errorf("[%s] expected acc.isSatisfied() to be %t", tc.description, tc.expectSatisfied)
		}
		if tc.expectFailed != acc.isFailed() {
			t.Errorf("[%s] expected acc.isFailed() to be %t", tc.description, tc.expectFailed)
		}
		for _, cpus := range tc.takeCPUs {
			availableCPUs := acc.details.CPUs()
			if cpus.Intersection(availableCPUs).Size() > 0 {
				t.Errorf("[%s] expected intersection of taken cpus [%s] and acc.details.CPUs() [%s] to be empty", tc.description, cpus, availableCPUs)
			}
			if !cpus.IsSubsetOf(acc.result) {
				t.Errorf("[%s] expected [%s] to be a subset of acc.result [%s]", tc.description, cpus, acc.result)
			}
		}
		expNumCPUsNeeded := tc.numCPUs - totalTaken
		if acc.numCPUsNeeded != expNumCPUsNeeded {
			t.Errorf("[%s] expected acc.numCPUsNeeded to be %d (got %d)", tc.description, expNumCPUsNeeded, acc.numCPUsNeeded)
		}
	}
}

func TestTakeByTopology(t *testing.T) {
	testCases := []struct {
		description   string
		topo          *CPUTopology
		availableCPUs cpuset.CPUSet
		numCPUs       int
		expErr        string
		expResult     cpuset.CPUSet
	}{
		{
			"take more cpus than are available from single socket with HT",
			topoSingleSocketHT,
			cpuset.NewCPUSet(0, 2, 4, 6),
			5,
			"not enough cpus available to satisfy request",
			cpuset.NewCPUSet(),
		},
		{
			"take zero cpus from single socket with HT",
			topoSingleSocketHT,
			cpuset.NewCPUSet(0, 1, 2, 3, 4, 5, 6, 7),
			0,
			"",
			cpuset.NewCPUSet(),
		},
		{
			"take one cpu from single socket with HT",
			topoSingleSocketHT,
			cpuset.NewCPUSet(0, 1, 2, 3, 4, 5, 6, 7),
			1,
			"",
			cpuset.NewCPUSet(0),
		},
		{
			"take one cpu from single socket with HT, some cpus are taken",
			topoSingleSocketHT,
			cpuset.NewCPUSet(1, 3, 5, 6, 7),
			1,
			"",
			cpuset.NewCPUSet(6),
		},
		{
			"take two cpus from single socket with HT",
			topoSingleSocketHT,
			cpuset.NewCPUSet(0, 1, 2, 3, 4, 5, 6, 7),
			2,
			"",
			cpuset.NewCPUSet(0, 4),
		},
		{
			"take three cpus from single socket with HT",
			topoSingleSocketHT,
			cpuset.NewCPUSet(1, 2, 3, 5, 6, 7),
			3,
			"",
			cpuset.NewCPUSet(1, 5, 3),
		},
		{
			"take four cpus from single socket with HT",
			topoSingleSocketHT,
			cpuset.NewCPUSet(1, 2, 3, 5, 6, 7),
			4,
			"",
			cpuset.NewCPUSet(1, 5, 3, 7),
		},
		{
			"take all cpus from single socket with HT",
			topoSingleSocketHT,
			cpuset.NewCPUSet(0, 1, 2, 3, 4, 5, 6, 7),
			8,
			"",
			cpuset.NewCPUSet(0, 1, 2, 3, 4, 5, 6, 7),
		},
		{
			"take two cpus from single socket with HT, only one core totally free",
			topoSingleSocketHT,
			cpuset.NewCPUSet(0, 1, 2, 3, 6),
			2,
			"",
			cpuset.NewCPUSet(2, 6),
		},
		{
			"take one cpu from dual socket with HT - core from Socket 0",
			topoDualSocketHT,
			cpuset.NewCPUSet(1, 2, 3, 4, 5, 7, 8, 9, 10, 11),
			1,
			"",
			cpuset.NewCPUSet(8),
		},
		{
			"take a socket of cpus from dual socket with HT",
			topoDualSocketHT,
			cpuset.NewCPUSet(0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15),
			8,
			"",
			cpuset.NewCPUSet(0, 8, 4, 12, 2, 10, 6, 14),
		},
	}

	for _, tc := range testCases {
		result, err := takeByTopology(tc.topo, tc.availableCPUs, tc.numCPUs)
		if tc.expErr != "" && err.Error() != tc.expErr {
			t.Errorf("expected error to be [%v] but it was [%v] in test \"%s\"", tc.expErr, err, tc.description)
		}
		if !result.Equals(tc.expResult) {
			t.Errorf("expected result [%s] to equal [%s] in test \"%s\"", result, tc.expResult, tc.description)
		}
	}
}

func TestContainerSort(t *testing.T) {
	type containerInfo struct {
		podName string
		containerName string
		requestCpu int
		cpusetpolicy CPUSetPolicy
		priorityClassValue int
	}

	l := []containerInfo{
		{
			podName: "1",
			containerName: "1",
			requestCpu: 16,
			cpusetpolicy: CPUSetExclusive,
			priorityClassValue: 1000,
		},
		{
			podName: "2",
			containerName: "1",
			requestCpu: 32,
			cpusetpolicy: CPUSetExclusive,
			priorityClassValue: 1000,
		},
		{
			podName: "3",
			containerName: "1",
			requestCpu: 16,
			cpusetpolicy: CPUSetShare,
			priorityClassValue: 1000,
		},
		{
			podName: "4",
			containerName: "1",
			requestCpu: 10,
			cpusetpolicy: CPUSetShare,
			priorityClassValue: 1000,
		},
		{
			podName: "4",
			containerName: "2",
			requestCpu: 11,
			cpusetpolicy: CPUSetShare,
			priorityClassValue: 1000,
		},
		{
			podName: "5",
			containerName: "1",
			requestCpu: 11,
			cpusetpolicy: CPUSetShare,
			priorityClassValue: 1000,
		},
		{
			podName: "5",
			containerName: "2",
			requestCpu: 11,
			cpusetpolicy: CPUSetShare,
			priorityClassValue: 1000,
		},
		{
			podName: "6",
			containerName: "1",
			requestCpu: 1,
			cpusetpolicy: CPUSetShare,
			priorityClassValue: 1000,
		},
		{
			podName: "7",
			containerName: "1",
			requestCpu: 1,
			cpusetpolicy: CPUSetExclusive,
			priorityClassValue: 1000,
		},
		{
			podName: "8",
			containerName: "1",
			requestCpu: 16,
			cpusetpolicy: CPUSetShare,
			priorityClassValue: 1000,
		},
	}

	sort.Slice(
		l,
		func(i, j int) bool {
			iPodName:=  l[i].podName
			jPodName:=  l[j].podName

			iContainerName:=  l[i].containerName
			jContainerName:=  l[j].containerName

			iRequestCpu := l[i].requestCpu
			jRequestCpu := l[j].requestCpu

			cpusPerCCD := 16

			iMod := iRequestCpu % cpusPerCCD
			jMod := jRequestCpu % cpusPerCCD

			iCpusetpolicy := l[i].cpusetpolicy
			jCpusetpolicy := l[j].cpusetpolicy

			iPriorityClassValue := l[i].priorityClassValue
			jPriorityClassValue := l[j].priorityClassValue

			return (iMod == 0 && jMod != 0) ||
				(iMod == jMod && iCpusetpolicy.Compare(jCpusetpolicy) == 1) ||
				(iMod == jMod && iCpusetpolicy.Compare(jCpusetpolicy) == 0 && iPriorityClassValue > jPriorityClassValue) ||
				(iMod == jMod && iCpusetpolicy.Compare(jCpusetpolicy) == 0 && iPriorityClassValue == jPriorityClassValue && iRequestCpu > jRequestCpu) ||
				(iMod == jMod && iCpusetpolicy.Compare(jCpusetpolicy) == 0 && iPriorityClassValue == jPriorityClassValue && iRequestCpu == jRequestCpu && iPodName > jPodName) ||
				(iMod == jMod && iCpusetpolicy.Compare(jCpusetpolicy) == 0 && iPriorityClassValue == jPriorityClassValue && iRequestCpu == jRequestCpu && iPodName == jPodName && iContainerName > jContainerName) ||
				(iMod != 0 && jMod != 0 && iMod != jMod && iCpusetpolicy.Compare(jCpusetpolicy) == 1) ||
				(iMod != 0 && jMod != 0 && iMod != jMod && iCpusetpolicy.Compare(jCpusetpolicy) == 0 && iPriorityClassValue > jPriorityClassValue) ||
				(iMod != 0 && jMod != 0 && iMod != jMod && iCpusetpolicy.Compare(jCpusetpolicy) == 0 && iPriorityClassValue == jPriorityClassValue && iRequestCpu > jRequestCpu) ||
				(iMod != 0 && jMod != 0 && iMod != jMod && iCpusetpolicy.Compare(jCpusetpolicy) == 0 && iPriorityClassValue == jPriorityClassValue && iRequestCpu == jRequestCpu && iPodName > jPodName) ||
				(iMod != 0 && jMod != 0 && iMod != jMod && iCpusetpolicy.Compare(jCpusetpolicy) == 0 && iPriorityClassValue == jPriorityClassValue && iRequestCpu == jRequestCpu && iPodName == jPodName && iContainerName > jContainerName)
		})

	t.Log(l)
}
