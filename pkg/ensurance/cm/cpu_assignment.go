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
// copied from kubernetes/pkg/kubelet/cm/cpumanager/cpu_assignment.go
package cm

import (
	"fmt"
	"sort"

	"k8s.io/klog/v2"

	"k8s.io/kubernetes/pkg/kubelet/cm/cpumanager/topology"
	"k8s.io/kubernetes/pkg/kubelet/cm/cpuset"
)

type cpuAccumulator struct {
	topo          *CPUTopology
	details       topology.CPUDetails
	ccdDetail     CPUDetails
	numCPUsNeeded int
	result        cpuset.CPUSet
}

func newCPUAccumulator(topo *CPUTopology, availableCPUs cpuset.CPUSet, numCPUs int) *cpuAccumulator {
	return &cpuAccumulator{
		topo:          topo,
		details:       topo.CPUDetails.KeepOnly(availableCPUs),
		ccdDetail:     topo.CPUCCDDetails.KeepOnly(availableCPUs),
		numCPUsNeeded: numCPUs,
		result:        cpuset.NewCPUSet(),
	}
}

func (a *cpuAccumulator) take(cpus cpuset.CPUSet) {
	a.result = a.result.Union(cpus)
	a.details = a.details.KeepOnly(a.details.CPUs().Difference(a.result))
	a.ccdDetail = a.ccdDetail.KeepOnly(a.ccdDetail.CPUs().Difference(a.result))
	a.numCPUsNeeded -= cpus.Size()
}

// Returns true if the supplied socket is fully available in `topoDetails`.
func (a *cpuAccumulator) isSocketFree(socketID int) bool {
	return a.details.CPUsInSockets(socketID).Size() == a.topo.CPUsPerSocket()
}

// Returns true if the supplied socket is fully available in `topoDetails`.
func (a *cpuAccumulator) isCCDFree(ccdId int) bool {
	return a.ccdDetail.CPUsInCCDs(ccdId).Size() == a.topo.CPUsPerCCD()
}

// Returns true if the supplied core is fully available in `topoDetails`.
func (a *cpuAccumulator) isCoreFree(coreID int) bool {
	return a.details.CPUsInCores(coreID).Size() == a.topo.CPUsPerCore()
}

// Returns free socket IDs as a slice sorted by:
// - socket ID, ascending.
func (a *cpuAccumulator) freeSockets() []int {
	return a.details.Sockets().Filter(a.isSocketFree).ToSlice()
}

// Returns free ccd IDs as a slice sorted by:
// - the number of whole available ccds on the socket, ascending
// - socket ID, ascending
// - ccd ID, ascending.
func (a *cpuAccumulator) freeCCDs() []int {
	socketIDs := a.details.Sockets().ToSliceNoSort()

	sort.Slice(socketIDs,
		func(i, j int) bool {
			iCCDs := a.CCDsInSockets(socketIDs[i]).Filter(a.isCCDFree)
			jCCDs := a.CCDsInSockets(socketIDs[j]).Filter(a.isCCDFree)
			return iCCDs.Size() < jCCDs.Size() || socketIDs[i] < socketIDs[j]
		})

	klog.Infof("socketIDs %v", socketIDs)

	ccdIDs := []int{}
	for _, s := range socketIDs {
		ccdIDs = append(ccdIDs, a.CCDsInSockets(s).Filter(a.isCCDFree).ToSlice()...)
	}
	return ccdIDs
}

// CCDsInSockets returns all of the core IDs associated with the given socket
// IDs in this cpuAccumulator.
func (a *cpuAccumulator) CCDsInSockets(ids ...int)cpuset.CPUSet {
	b := cpuset.NewBuilder()
	for _, id := range ids {
		for i, info := range a.ccdDetail {
			cpuInfo, ok := a.details[i]
			if !ok {
				continue
			}
			if cpuInfo.SocketID == id {
				b.Add(info.CCDID)
			}
		}
	}
	klog.Infof("ccd %v in socket %v", b.Result().ToSliceNoSort(), ids)
	return b.Result()
}

// Returns core IDs as a slice sorted by:
// - ccd affinity with result
// - the number of whole available cores on the socket, ascending
// - the number of whole available cores on the ccds, ascending
// - socket ID, ascending
// - ccd ID, ascending
// - core ID, ascending
func (a *cpuAccumulator) freeCores() []int {
	ccdIDs := a.ccdDetail.CCDs().ToSlice()
	sort.Slice(ccdIDs,
		func(i, j int) bool {
			iCCD := ccdIDs[i]
			jCCD := ccdIDs[j]

			iCPUs := a.topo.CPUCCDDetails.CPUsInCCDs(iCCD).ToSlice()
			jCPUs := a.topo.CPUCCDDetails.CPUsInCCDs(jCCD).ToSlice()

			iSocket := a.topo.CPUDetails[iCPUs[0]].SocketID
			jSocket := a.topo.CPUDetails[jCPUs[0]].SocketID

			iSocketCoresFree := a.details.CoresInSockets(iSocket).Filter(a.isCoreFree)
			jSocketCoresFree := a.details.CoresInSockets(jSocket).Filter(a.isCoreFree)

			// Compute the number of CPUs in the result reside on the same ccd
			// as each core.
			iCCdColoScore := a.topo.CPUCCDDetails.CPUsInCCDs(iCCD).Intersection(a.result).Size()
			jCCdColoScore := a.topo.CPUCCDDetails.CPUsInCCDs(jCCD).Intersection(a.result).Size()

			iCCDCoresFree := a.CoresInCCDs(iCCD).Filter(a.isCoreFree)
			jCCDCoresFree := a.CoresInCCDs(jCCD).Filter(a.isCoreFree)

			return iCCdColoScore > jCCdColoScore ||
				iSocketCoresFree.Size() < jSocketCoresFree.Size() ||
				iCCDCoresFree.Size() < jCCDCoresFree.Size() ||
				iSocket < jSocket ||
				iCCD < jCCD
		})

	coreIDs := []int{}
	for _, s := range ccdIDs {
		coreIDs = append(coreIDs, a.CoresInCCDs(s).Filter(a.isCoreFree).ToSlice()...)
	}
	return coreIDs
}

// CCDsInSockets returns all of the core IDs associated with the given socket
// IDs in this cpuAccumulator.
func (a *cpuAccumulator) CoresInCCDs(ids ...int)cpuset.CPUSet {
	b := cpuset.NewBuilder()
	for _, id := range ids {
		for i, info := range a.ccdDetail {
			cpuInfo, ok := a.details[i]
			if !ok {
				continue
			}
			if info.CCDID == id {
				b.Add(cpuInfo.CoreID)
			}
		}
	}
	return b.Result()
}

// Returns CPU IDs as a slice sorted by:
// - socket affinity with result
// - ccd affinity with result
// - number of CPUs available on the same socket
// - number of CPUs available on the same ccd
// - number of CPUs available on the same core
// - socket ID.
// - ccd ID.
// - core ID.
func (a *cpuAccumulator) freeCPUs() []int {
	result := []int{}
	cores := a.details.Cores().ToSlice()

	sort.Slice(
		cores,
		func(i, j int) bool {
			iCore := cores[i]
			jCore := cores[j]

			iCPUs := a.topo.CPUDetails.CPUsInCores(iCore).ToSlice()
			jCPUs := a.topo.CPUDetails.CPUsInCores(jCore).ToSlice()

			iSocket := a.topo.CPUDetails[iCPUs[0]].SocketID
			jSocket := a.topo.CPUDetails[jCPUs[0]].SocketID

			iCCD := a.topo.CPUCCDDetails[iCPUs[0]].CCDID
			jCCD := a.topo.CPUCCDDetails[jCPUs[0]].CCDID

			// Compute the number of CPUs in the result reside on the same socket
			// as each core.
			iSocketColoScore := a.topo.CPUDetails.CPUsInSockets(iSocket).Intersection(a.result).Size()
			jSocketColoScore := a.topo.CPUDetails.CPUsInSockets(jSocket).Intersection(a.result).Size()

			// Compute the number of CPUs in the result reside on the same ccd
			// as each core.
			iCCdColoScore := a.topo.CPUCCDDetails.CPUsInCCDs(iCCD).Intersection(a.result).Size()
			jCCdColoScore := a.topo.CPUCCDDetails.CPUsInCCDs(jCCD).Intersection(a.result).Size()

			// Compute the number of available CPUs available on the same socket
			// as each core.
			iSocketFreeScore := a.details.CPUsInSockets(iSocket).Size()
			jSocketFreeScore := a.details.CPUsInSockets(jSocket).Size()

			// Compute the number of available CPUs available on the same ccd
			// as each core.
			iCCDFreeScore := a.ccdDetail.CPUsInCCDs(iCCD).Size()
			jCCDFreeScore := a.ccdDetail.CPUsInCCDs(jCCD).Size()

			// Compute the number of available CPUs on each core.
			iCoreFreeScore := a.details.CPUsInCores(iCore).Size()
			jCoreFreeScore := a.details.CPUsInCores(jCore).Size()

			return iSocketColoScore > jSocketColoScore ||
				iCCdColoScore > jCCdColoScore ||
				iSocketFreeScore < jSocketFreeScore ||
				iCCDFreeScore < jCCDFreeScore ||
				iCoreFreeScore < jCoreFreeScore ||
				iSocket < jSocket ||
				iCCD < jCCD ||
				iCore < jCore
		})

	// For each core, append sorted CPU IDs to result.
	for _, core := range cores {
		result = append(result, a.details.CPUsInCores(core).ToSlice()...)
	}
	return result
}

func (a *cpuAccumulator) needs(n int) bool {
	return a.numCPUsNeeded >= n
}

func (a *cpuAccumulator) isSatisfied() bool {
	return a.numCPUsNeeded < 1
}

func (a *cpuAccumulator) isFailed() bool {
	return a.numCPUsNeeded > a.details.CPUs().Size()
}

func takeByTopology(topo *CPUTopology, availableCPUs cpuset.CPUSet, numCPUs int) (cpuset.CPUSet, error) {
	acc := newCPUAccumulator(topo, availableCPUs, numCPUs)
	if acc.isSatisfied() {
		return acc.result, nil
	}
	if acc.isFailed() {
		return cpuset.NewCPUSet(), fmt.Errorf("not enough cpus available to satisfy request")
	}

	// Algorithm: topology-aware best-fit
	// 1. Acquire whole sockets, if available and the container requires at
	//    least a socket's-worth of CPUs.
	if acc.needs(acc.topo.CPUsPerSocket()) {
		for _, s := range acc.freeSockets() {
			klog.V(4).Infof("[Advancedcpumanager] takeByTopology: claiming socket [%d]", s)
			acc.take(acc.details.CPUsInSockets(s))
			if acc.isSatisfied() {
				return acc.result, nil
			}
			if !acc.needs(acc.topo.CPUsPerSocket()) {
				break
			}
		}
	}

	// 2. Acquire whole ccds, if available and the container requires at least
	//    a ccd's-worth of CPUs.
	if acc.needs(acc.topo.CPUsPerCCD()) {
		for _, c := range acc.freeCCDs() {
			klog.V(4).Infof("[Advancedcpumanager] takeByTopology: claiming ccd [%d]", c)
			acc.take(acc.ccdDetail.CPUsInCCDs(c))
			if acc.isSatisfied() {
				return acc.result, nil
			}
			if !acc.needs(acc.topo.CPUsPerCCD()) {
				break
			}
		}
	}

	// 3. Acquire whole cores, if available and the container requires at least
	//    a core's-worth of CPUs.
	if acc.needs(acc.topo.CPUsPerCore()) {
		cores := acc.freeCores()
		sort.Slice(cores,
			func(i, j int) bool {
				iCore := cores[i]
				jCore := cores[j]

				iCPUs := acc.topo.CPUDetails.CPUsInCores(iCore).ToSlice()
				jCPUs := acc.topo.CPUDetails.CPUsInCores(jCore).ToSlice()

				iCCD := acc.topo.CPUCCDDetails[iCPUs[0]].CCDID
				jCCD := acc.topo.CPUCCDDetails[jCPUs[0]].CCDID

				iCCDFreeScore := acc.ccdDetail.CPUsInCCDs(iCCD).Size()
				jCCDFreeScore := acc.ccdDetail.CPUsInCCDs(jCCD).Size()

				iScore := iCCDFreeScore - acc.numCPUsNeeded
				jScore := jCCDFreeScore - acc.numCPUsNeeded

				return (iScore >= 0 && jScore >= 0 && iScore < jScore) ||
					(iScore <= 0 && jScore < 0 && iScore > jScore) ||
					(iScore >0 && jScore < 0)
			})
		for _, c := range cores {
			klog.V(4).Infof("[Advancedcpumanager] takeByTopology: claiming core [%d]", c)
			acc.take(acc.details.CPUsInCores(c))
			if acc.isSatisfied() {
				return acc.result, nil
			}
			if !acc.needs(acc.topo.CPUsPerCore()) {
				break
			}
		}
	}

	// 4. Acquire single threads, preferring to fill partially-allocated cores
	//    on the same sockets as the whole cores we have already taken in this
	//    allocation.
	for _, c := range acc.freeCPUs() {
		klog.V(4).Infof("[Advancedcpumanager] takeByTopology: claiming CPU [%d]", c)
		if acc.needs(1) {
			acc.take(cpuset.NewCPUSet(c))
		}
		if acc.isSatisfied() {
			return acc.result, nil
		}
	}

	return cpuset.NewCPUSet(), fmt.Errorf("failed to allocate cpus")
}
