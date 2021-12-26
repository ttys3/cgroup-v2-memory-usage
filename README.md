# cgroup-v2-memory-usage

related issue:  https://github.com/containers/podman/issues/12702

related PR:

https://github.com/containers/podman/pull/12703

https://github.com/containers/common/pull/870


## podman stats shows incorrect memory usage on cgroup v2 Linux


**Is this a BUG REPORT or FEATURE REQUEST? (leave only one on its own line)**

/kind bug

related PR:

https://github.com/containers/podman/pull/12703

https://github.com/containers/common/pull/870

**Description**

`podman stats` shows incorrect memory usage

**Steps to reproduce the issue:**

1.  run kafka pod

prepare data dir:

```
sudo mkdir /var/lib/zookeeper/log
sudo mkdir /var/lib/zookeeper/data
sudo mkdir /var/lib/kafka/data

sudo chown -R 1000:1000 /var/lib/zookeeper
sudo chown -R 1000:1000 /var/lib/kafka
```

`sudo podman play kube kafka.pod.yaml`

kafka.pod.yaml content:

```yaml
# Save the output of this file and use kubectl create -f to import
# it into Kubernetes.
#
# Created with podman-3.4.4
apiVersion: v1
kind: Pod
metadata:
  creationTimestamp: "2021-12-25T12:26:19Z"
  labels:
    app: kafka
  name: kafka
spec:
  containers:
  - env:
    - name: ZOOKEEPER_CLIENT_PORT
      value: "2181"
    - name: ZOOKEEPER_TICK_TIME
      value: "2000"
    image: docker.io/confluentinc/cp-zookeeper:7.0.1
    name: zookeeper
    ports:
    - containerPort: 9092
      hostPort: 9092
    resources: {}
    securityContext:
      capabilities:
        drop:
        - CAP_MKNOD
        - CAP_NET_RAW
        - CAP_AUDIT_WRITE
    volumeMounts:
    - mountPath: /var/lib/zookeeper/data
      name: var-lib-zookeeper-data-host-0
    - mountPath: /var/lib/zookeeper/log
      name: var-lib-zookeeper-log-host-1
  - env:
    - name: KAFKA_ADVERTISED_LISTENERS
      value: PLAINTEXT://localhost:9092,PLAINTEXT_INTERNAL://localhost:29092
    - name: KAFKA_TRANSACTION_STATE_LOG_MIN_ISR
      value: "1"
    - name: KAFKA_TRANSACTION_STATE_LOG_REPLICATION_FACTOR
      value: "1"
    - name: KAFKA_BROKER_ID
      value: "1"
    - name: KAFKA_LISTENER_SECURITY_PROTOCOL_MAP
      value: PLAINTEXT:PLAINTEXT,PLAINTEXT_INTERNAL:PLAINTEXT
    - name: KAFKA_ZOOKEEPER_CONNECT
      value: localhost:2181
    - name: KAFKA_OFFSETS_TOPIC_REPLICATION_FACTOR
      value: "1"
    image: docker.io/confluentinc/cp-kafka:7.0.1
    name: broker
    resources: {}
    securityContext:
      capabilities:
        drop:
        - CAP_MKNOD
        - CAP_NET_RAW
        - CAP_AUDIT_WRITE
    volumeMounts:
    - mountPath: /var/lib/kafka/data
      name: var-lib-kafka-data-host-0
  restartPolicy: Never
  volumes:
  - hostPath:
      path: /var/lib/zookeeper/log
      type: Directory
    name: var-lib-zookeeper-log-host-1
  - hostPath:
      path: /var/lib/kafka/data
      type: Directory
    name: var-lib-kafka-data-host-0
  - hostPath:
      path: /var/lib/zookeeper/data
      type: Directory
    name: var-lib-zookeeper-data-host-0
status: {}
```

2.  use a simple producer send message to the broker (let's make lots network traffic):

`go run producer.go`

```go
// producer.go
package main

import (
	"fmt"
	"log"
	"math/rand"
	"os"
	"strings"

	"github.com/confluentinc/confluent-kafka-go/kafka"
)

func main() {
	conf := kafka.ConfigMap{
		"bootstrap.servers": "localhost:9092",
		"broker.address.family": "v4",
	}

	topic := "demo"
	p, err := kafka.NewProducer(&conf)
	if err != nil {
		fmt.Printf("Failed to create producer: %s", err)
		os.Exit(1)
	}

	// Go-routine to handle message delivery reports and
	// possibly other event types (errors, stats, etc)
	go func() {
		for e := range p.Events() {
			switch ev := e.(type) {
			case *kafka.Message:
				if ev.TopicPartition.Error != nil {
					fmt.Printf("Failed to deliver message: %v\n", ev.TopicPartition)
				} else {
					fmt.Printf("Produced event to topic %s\n", *ev.TopicPartition.Topic)
				}
			}
		}
	}()

	items := [...]string{"book", "alarm clock", "t-shirts", "gift card", "batteries"}

	batchSize := 10000
	n := 0
	for {
		data := strings.Repeat(items[rand.Intn(len(items))], 10240)
		err := p.Produce(&kafka.Message{
			TopicPartition: kafka.TopicPartition{Topic: &topic, Partition: kafka.PartitionAny},
			// Key:            []byte(key),
			Value: []byte(data),
		}, nil)
		if err != nil {
			log.Printf("err=%v", err)
		}
		if n%batchSize == 0 {
			// Wait for all messages to be delivered
			p.Flush(15 * 1000)
			n = 0
		}
		n++
	}

	// Wait for all messages to be delivered
	p.Flush(15 * 1000)
	p.Close()
}
```


3. run `podman stats`, you will see the kafka broker container memory `USAGE` increasing very fast, you'll see it goes to GBs

but if you `exec` into the contianer, and use `top` show the real memory usage, it only used `344M` maybe

Just wait small minutes in order to generate some network traffic for the container.

**Describe the results you received:**

The memory increase very fast, and incorrect.

```
ID            NAME                CPU %       MEM USAGE / LIMIT  MEM %       NET IO             BLOCK IO           PIDS        CPU TIME      AVG CPU %
ba2f357a1a56  broker          4.83%       8.352GB / 33.6GB     29.17%      5.816GB / 15.02GB  1.445GB / 60.05GB  681         2m15.913798s  3.22%
```

**Describe the results you expected:**

the memory usage should match the one `RSS` in `top` command in the container.

```
ID            NAME                CPU %       MEM USAGE / LIMIT  MEM %       NET IO             BLOCK IO           PIDS        CPU TIME      AVG CPU %
ba2f357a1a56  broker          4.83%       344M / 33.6GB     29.17%      5.816GB / 15.02GB  1.445GB / 60.05GB  681         2m15.913798s  3.22%
```

**Additional information you deem important (e.g. issue happens only occasionally):**

**Output of `podman version`:**

```
Version:      3.4.4
API Version:  3.4.4
Go Version:   go1.17.5
Git Commit:   7feb0d944f86dc9861bba398b7505fe9bb2ff4bb
Built:        Sat Dec 18 08:51:40 2021
OS/Arch:      linux/amd64

```

**Output of `podman info --debug`:**

```
host:
  arch: amd64
  buildahVersion: 1.23.1
  cgroupControllers:
  - cpuset
  - cpu
  - io
  - memory
  - hugetlb
  - pids
  - rdma
  - misc
  cgroupManager: systemd
  cgroupVersion: v2
  conmon:
    package: /usr/bin/conmon is owned by conmon 1:2.0.31-1
    path: /usr/bin/conmon
    version: 'conmon version 2.0.31, commit: 7e7eb74e52abf65a6d46807eeaea75425cc8a36c'
  cpus: 16
  distribution:
    distribution: arch
    version: unknown
  eventLogger: journald
  hostname: wudeng
  idMappings:
    gidmap: null
    uidmap: null
  kernel: 5.15.11-arch2-1
  linkmode: dynamic
  logDriver: journald
  memFree: 2378579968
  memTotal: 33595195392
  ociRuntime:
    name: crun
    package: /usr/bin/crun is owned by crun 1.3-3
    path: /usr/bin/crun
    version: |-
      crun version 1.3
      commit: 4f6c8e0583c679bfee6a899c05ac6b916022561b
      spec: 1.0.0
      +SYSTEMD +SELINUX +APPARMOR +CAP +SECCOMP +EBPF +CRIU +YAJL
  os: linux
  remoteSocket:
    exists: true
    path: /run/podman/podman.sock
  security:
    apparmorEnabled: false
    capabilities: CAP_CHOWN,CAP_DAC_OVERRIDE,CAP_FOWNER,CAP_FSETID,CAP_KILL,CAP_NET_BIND_SERVICE,CAP_SETFCAP,CAP_SETGID,CAP_SETPCAP,CAP_SETUID,CAP_SYS_CHROOT
    rootless: false
    seccompEnabled: true
    seccompProfilePath: /etc/containers/seccomp.json
    selinuxEnabled: false
  serviceIsRemote: false
  slirp4netns:
    executable: /usr/bin/slirp4netns
    package: /usr/bin/slirp4netns is owned by slirp4netns 1.1.12-1
    version: |-
      slirp4netns version 1.1.12
      commit: 7a104a101aa3278a2152351a082a6df71f57c9a3
      libslirp: 4.6.1
      SLIRP_CONFIG_VERSION_MAX: 3
      libseccomp: 2.5.3
  swapFree: 9800249344
  swapTotal: 17179865088
  uptime: 24h 2m 33.2s (Approximately 1.00 days)
plugins:
  log:
  - k8s-file
  - none
  - journald
  network:
  - bridge
  - macvlan
  volume:
  - local
registries:
  hub.k8s.lan:
    Blocked: false
    Insecure: true
    Location: hub.k8s.lan
    MirrorByDigestOnly: false
    Mirrors: null
    Prefix: hub.k8s.lan
  search:
  - docker.io
store:
  configFile: /etc/containers/storage.conf
  containerStore:
    number: 24
    paused: 0
    running: 7
    stopped: 17
  graphDriverName: overlay
  graphOptions:
    overlay.mountopt: nodev
  graphRoot: /var/lib/containers/storage
  graphStatus:
    Backing Filesystem: extfs
    Native Overlay Diff: "false"
    Supports d_type: "true"
    Using metacopy: "true"
  imageStore:
    number: 26
  runRoot: /run/containers/storage
  volumePath: /var/lib/containers/storage/volumes
version:
  APIVersion: 3.4.4
  Built: 1639788700
  BuiltTime: Sat Dec 18 08:51:40 2021
  GitCommit: 7feb0d944f86dc9861bba398b7505fe9bb2ff4bb
  GoVersion: go1.17.5
  OsArch: linux/amd64
  Version: 3.4.4

```

**Package info (e.g. output of `rpm -q podman` or `apt list podman`):**

```
Name            : podman
Version         : 3.4.4-1
Description     : Tool and library for running OCI-based containers in pods
Architecture    : x86_64
URL             : https://github.com/containers/podman
Licenses        : Apache
Groups          : None
Provides        : None
Depends On      : cni-plugins  conmon  containers-common  crun  fuse-overlayfs  iptables  libdevmapper.so=1.02-64  libgpgme.so=11-64  libseccomp.so=2-64  slirp4netns
Optional Deps   : apparmor: for AppArmor support
                  btrfs-progs: support btrfs backend devices [installed]
                  catatonit: --init flag support
                  podman-docker: for Docker-compatible CLI
Required By     : cockpit-podman  nomad-driver-podman  podman-compose
Optional For    : None
Conflicts With  : None
Replaces        : None
Installed Size  : 72.79 MiB
Packager        : David Runge <dvzrv@archlinux.org>
Build Date      : Fri 10 Dec 2021 02:30:40 AM CST
Install Date    : Fri 10 Dec 2021 09:04:48 AM CST
Install Reason  : Explicitly installed
Install Script  : No
Validated By    : Signature

```

**Have you tested with the latest version of Podman and have you checked the Podman Troubleshooting Guide? (https://github.com/containers/podman/blob/master/troubleshooting.md)**


Yes

**Additional environment details (AWS, VirtualBox, physical, etc.):**

**cgroup v2**

kernel `5.15.11-arch2-1 #1 SMP PREEMPT Wed, 22 Dec 2021 09:23:54 +0000 x86_64 GNU/Linux`
