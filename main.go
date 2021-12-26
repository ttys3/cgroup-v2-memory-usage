package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"path/filepath"
	"strconv"
	"strings"
)

const cgroupRoot = "/sys/fs/cgroup"

func main() {
	var memoryRoot string
	filenames := map[string]string{}

	// get container cgroup path:
	// sudo podman inspect containerName | rg CgroupPath
	// /machine.slice/libpod-ba2f357a1a56ae4c263d4f5e8d46e12ceef89a08d7333ca29278e4f119e1b65c.scope

	ctrCgroupPath := "/machine.slice/libpod-92ebc3daa3d52be6dd137575e0e247e40c732cfed0cfb78c3a037edb2c3a11ba.scope"
	memoryRoot = filepath.Join(cgroupRoot, ctrCgroupPath)
	filenames["usage"] = "memory.stat"
	filenames["limit"] = "memory.max"

	anon, err := readFileByKeyAsUint64(filepath.Join(memoryRoot, filenames["usage"]), "anon")
	if err != nil {
		log.Fatal(err)
	}

	inactiveAnon, err := readFileByKeyAsUint64(filepath.Join(memoryRoot, filenames["usage"]), "inactive_anon")
	if err != nil {
		log.Fatal(err)
	}
	activeAnon, err := readFileByKeyAsUint64(filepath.Join(memoryRoot, filenames["usage"]), "active_anon")
	if err != nil {
		log.Fatal(err)
	}
	usage := anon
	GiB := float64(1024 * 1024 * 1024)
	log.Printf("memory usage: anon=%.2f GiB anon_podman_human=%.2f GB  inactive_anon=%.2f GiB active_anon=%.2f GiB",
		float64(usage)/GiB, float64(usage)/float64(1000*1000*1000), float64(inactiveAnon)/GiB, float64(activeAnon)/GiB)
	// memory usage: anon=1.24 GiB anon_podman_human=1.33 GB  inactive_anon=1.24 GiB active_anon=0.00 GiB

	var metrics Metrics
	mc := getMemoryHandler()
	err = mc.Stat(&CgroupControl{
		cgroup2: true,
		path:    ctrCgroupPath,
		systemd: false,
	}, &metrics)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Metrics=%+v\n", metrics)

	log.Printf("Metrics.Usage=%.2f Limit=%.2f\n", float64(metrics.Memory.Usage.Usage)/GiB, float64(metrics.Memory.Usage.Limit)/GiB)
}

func readFileByKeyAsUint64(path, key string) (uint64, error) {
	content, err := ioutil.ReadFile(path)
	if err != nil {
		return 0, err
	}
	for _, line := range strings.Split(string(content), "\n") {
		fields := strings.SplitN(line, " ", 2)
		if fields[0] == key {
			v := strings.TrimSpace(string(fields[1]))
			if v == "max" {
				return math.MaxUint64, nil
			}
			ret, err := strconv.ParseUint(v, 10, 64)
			if err != nil {
				return ret, fmt.Errorf("parse %s from %s, err=%v", v, path, err)
			}
			return ret, nil
		}
	}

	return 0, fmt.Errorf("no key named %s from %s", key, path)
}

func cleanString(s string) string {
	return strings.Trim(s, "\n")
}

func readFileAsUint64(path string) (uint64, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return 0, err
	}
	v := cleanString(string(data))
	if v == "max" {
		return math.MaxUint64, nil
	}
	ret, err := strconv.ParseUint(v, 10, 64)
	if err != nil {
		return ret, fmt.Errorf("parse %s from %s err=%v", v, path, err)
	}
	return ret, nil
}

type memHandler struct{}

func getMemoryHandler() *memHandler {
	return &memHandler{}
}

// CgroupControl controls a cgroup hierarchy
type CgroupControl struct {
	cgroup2 bool
	path    string
	systemd bool
}

// MemoryUsage keeps stats for the memory usage
type MemoryUsage struct {
	Usage uint64
	Limit uint64
}

// MemoryMetrics keeps usage stats for the memory cgroup controller
type MemoryMetrics struct {
	Usage MemoryUsage
}

// Metrics keeps usage stats for the cgroup controllers
type Metrics struct {
	//CPU    CPUMetrics
	//Blkio  BlkioMetrics
	Memory MemoryMetrics
	//Pids   PidsMetrics
}

// Stat fills a metrics structure with usage stats for the controller
func (c *memHandler) Stat(ctr *CgroupControl, m *Metrics) error {
	var err error
	usage := MemoryUsage{}

	var memoryRoot string
	var limitFilename string

	if ctr.cgroup2 {
		memoryRoot = filepath.Join(cgroupRoot, ctr.path)
		limitFilename = "memory.max"
		if usage.Usage, err = readFileByKeyAsUint64(filepath.Join(memoryRoot, "memory.stat"), "anon"); err != nil {
			return err
		}
	} /* else {
		memoryRoot = ctr.getCgroupv1Path(Memory)
		limitFilename = "memory.limit_in_bytes"
		if usage.Usage, err = readFileAsUint64(filepath.Join(memoryRoot, "memory.usage_in_bytes")); err != nil {
			return err
		}
	}
	*/
	usage.Limit, err = readFileAsUint64(filepath.Join(memoryRoot, limitFilename))
	if err != nil {
		return err
	}

	m.Memory = MemoryMetrics{Usage: usage}
	return nil
}
