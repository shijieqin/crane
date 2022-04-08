package cm

import (
	"fmt"
	info "github.com/google/cadvisor/info/v1"
	"k8s.io/klog/v2"
	"k8s.io/kubernetes/pkg/kubelet/cm/cpumanager/topology"
	"k8s.io/kubernetes/pkg/kubelet/cm/cpuset"
	"path/filepath"

	"github.com/gocrane/crane/pkg/utils"
)

type CPUDetails map[int]CPUInfo

type CPUInfo struct {
	CCDID int
}

type CCDTopology struct {
	NumCCDs int
	CPUCCDDetails CPUDetails
}

type CPUTopology struct {
	*topology.CPUTopology
	*CCDTopology
}

func (topo *CPUTopology) CPUsPerCCD() int {
	if topo.NumCCDs == 0 {
		return 0
	}
	return topo.NumCPUs / topo.NumCCDs
}

// KeepOnly returns a new CPUDetails object with only the supplied cpus.
func (d CPUDetails) KeepOnly(cpus cpuset.CPUSet) CPUDetails {
	result := CPUDetails{}
	for cpu, info := range d {
		if cpus.Contains(cpu) {
			result[cpu] = info
		}
	}
	return result
}

// CPUs returns all of the logical CPU IDs in this CPUDetails.
func (d CPUDetails) CPUs() cpuset.CPUSet {
	b := cpuset.NewBuilder()
	for cpuID := range d {
		b.Add(cpuID)
	}
	return b.Result()
}

// CPUsInCCDs returns all of the logical CPU IDs associated with the given
// ccd IDs in this CPUDetails.
func (d CPUDetails) CPUsInCCDs(ids ...int) cpuset.CPUSet {
	b := cpuset.NewBuilder()
	for _, id := range ids {
		for cpu, info := range d {
			if info.CCDID == id {
				b.Add(cpu)
			}
		}
	}
	return b.Result()
}

// CCDs returns all of the ccd IDs associated with the CPUs in this
// CPUDetails.
func (d CPUDetails) CCDs() cpuset.CPUSet {
	b := cpuset.NewBuilder()
	for _, info := range d {
		b.Add(info.CCDID)
	}
	return b.Result()
}

func Discover(machineInfo *info.MachineInfo) (*CPUTopology, error) {
	commonTopo, err := topology.Discover(machineInfo)
	if err != nil {
		return nil, err
	}
	ccdTopo, err := discoverCCD(commonTopo.CPUDetails.CPUs())
	if err != nil {
		klog.Errorf("Failed to discover ccd topology: %v", err)
	}
	return &CPUTopology{commonTopo, ccdTopo}, nil
}

func discoverCCD(cpuSet cpuset.CPUSet) (*CCDTopology, error) {
	ccdCpuSet := make([]cpuset.CPUSet, 0)
	L:
		for _, cpuId := range cpuSet.ToSlice() {
			for _, cpuSet := range ccdCpuSet{
				if cpuSet.Contains(cpuId) {
					continue L
				}
			}
			cpuSet, err := getShareCpuList(cpuId)
			if err != nil {
				return nil, err
			}
			ccdCpuSet = append(ccdCpuSet, cpuSet)
		}

	cPUDetails := make(map[int]CPUInfo)

	for ccdID, cpuSet := range ccdCpuSet{
		for _, cpuId := range cpuSet.ToSlice() {
			cPUDetails[cpuId] = CPUInfo{CCDID: ccdID}
		}
	}

	return &CCDTopology{len(ccdCpuSet), cPUDetails}, nil
}

func getShareCpuList(cpuId int) (cpuset.CPUSet, error) {
	sharedCpuListPath := filepath.Join("/sys/devices/system/cpu", fmt.Sprintf("cpu%d", cpuId), "/cache/index3/shared_cpu_list")
	shares, err := utils.ReadLines(sharedCpuListPath)
	if err != nil {
		return cpuset.NewCPUSet(), err
	}
	if len(shares) != 1 {
		return cpuset.NewCPUSet(), fmt.Errorf("len lines in shared_cpu_list is not 1")
	}

	cpus, err := utils.ParseRangeToSlice(shares[0])
	if err != nil {
		return cpuset.NewCPUSet(), err
	}

	return cpuset.NewCPUSet(cpus...), err
}