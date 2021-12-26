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
	memoryRoot = filepath.Join(cgroupRoot, "/machine.slice/libpod-ba2f357a1a56ae4c263d4f5e8d46e12ceef89a08d7333ca29278e4f119e1b65c.scope")
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
