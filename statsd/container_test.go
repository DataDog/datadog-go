//go:build linux
// +build linux

package statsd

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseContainerID(t *testing.T) {
	for input, expectedResult := range map[string]string{
		`other_line
10:hugetlb:/kubepods/burstable/podfd52ef25-a87d-11e9-9423-0800271a638e/8c046cb0b72cd4c99f51b5591cd5b095967f58ee003710a45280c28ee1a9c7fa
9:cpuset:/kubepods/burstable/podfd52ef25-a87d-11e9-9423-0800271a638e/8c046cb0b72cd4c99f51b5591cd5b095967f58ee003710a45280c28ee1a9c7fa
8:pids:/kubepods/burstable/podfd52ef25-a87d-11e9-9423-0800271a638e/8c046cb0b72cd4c99f51b5591cd5b095967f58ee003710a45280c28ee1a9c7fa
7:freezer:/kubepods/burstable/podfd52ef25-a87d-11e9-9423-0800271a638e/8c046cb0b72cd4c99f51b5591cd5b095967f58ee003710a45280c28ee1a9c7fa
6:cpu,cpuacct:/kubepods/burstable/podfd52ef25-a87d-11e9-9423-0800271a638e/8c046cb0b72cd4c99f51b5591cd5b095967f58ee003710a45280c28ee1a9c7fa
5:perf_event:/kubepods/burstable/podfd52ef25-a87d-11e9-9423-0800271a638e/8c046cb0b72cd4c99f51b5591cd5b095967f58ee003710a45280c28ee1a9c7fa
4:blkio:/kubepods/burstable/podfd52ef25-a87d-11e9-9423-0800271a638e/8c046cb0b72cd4c99f51b5591cd5b095967f58ee003710a45280c28ee1a9c7fa
3:devices:/kubepods/burstable/podfd52ef25-a87d-11e9-9423-0800271a638e/8c046cb0b72cd4c99f51b5591cd5b095967f58ee003710a45280c28ee1a9c7fa
2:net_cls,net_prio:/kubepods/burstable/podfd52ef25-a87d-11e9-9423-0800271a638e/8c046cb0b72cd4c99f51b5591cd5b095967f58ee003710a45280c28ee1a9c7fa`: "8c046cb0b72cd4c99f51b5591cd5b095967f58ee003710a45280c28ee1a9c7fa",
		"10:hugetlb:/kubepods/burstable/podfd52ef25-a87d-11e9-9423-0800271a638e/8c046cb0b72cd4c99f51b5591cd5b095967f58ee003710a45280c28ee1a9c7fa": "8c046cb0b72cd4c99f51b5591cd5b095967f58ee003710a45280c28ee1a9c7fa",
		"10:hugetlb:/kubepods": "",
		"11:hugetlb:/ecs/55091c13-b8cf-4801-b527-f4601742204d/432624d2150b349fe35ba397284dea788c2bf66b885d14dfc1569b01890ca7da": "432624d2150b349fe35ba397284dea788c2bf66b885d14dfc1569b01890ca7da",
		"1:name=systemd:/docker/34dc0b5e626f2c5c4c5170e34b10e7654ce36f0fcd532739f4445baabea03376":                               "34dc0b5e626f2c5c4c5170e34b10e7654ce36f0fcd532739f4445baabea03376",
		"1:name=systemd:/uuid/34dc0b5e-626f-2c5c-4c51-70e34b10e765":                                                             "34dc0b5e-626f-2c5c-4c51-70e34b10e765",
		"1:name=systemd:/ecs/34dc0b5e626f2c5c4c5170e34b10e765-1234567890":                                                       "34dc0b5e626f2c5c4c5170e34b10e765-1234567890",
		"1:name=systemd:/docker/34dc0b5e626f2c5c4c5170e34b10e7654ce36f0fcd532739f4445baabea03376.scope":                         "34dc0b5e626f2c5c4c5170e34b10e7654ce36f0fcd532739f4445baabea03376",
		`1:name=systemd:/nope
2:pids:/docker/34dc0b5e626f2c5c4c5170e34b10e7654ce36f0fcd532739f4445baabea03376
3:cpu:/invalid`: "34dc0b5e626f2c5c4c5170e34b10e7654ce36f0fcd532739f4445baabea03376",
	} {
		id := parseContainerID(strings.NewReader(input))
		assert.Equal(t, expectedResult, id)
	}
}

func TestReadContainerID(t *testing.T) {
	cid := "8c046cb0b72cd4c99f51b5591cd5b095967f58ee003710a45280c28ee1a9c7fa"
	cgroupContents := "10:hugetlb:/kubepods/burstable/podfd52ef25-a87d-11e9-9423-0800271a638e/" + cid

	tmpFile, err := ioutil.TempFile(os.TempDir(), "fake-cgroup-")
	assert.NoError(t, err)

	defer os.Remove(tmpFile.Name())

	_, err = io.WriteString(tmpFile, cgroupContents)
	assert.NoError(t, err)

	err = tmpFile.Close()
	assert.NoError(t, err)

	actualCID := readContainerID(tmpFile.Name())
	assert.Equal(t, cid, actualCID)
}

func TestParseMountinfo(t *testing.T) {
	for input, expectedResult := range map[string]string{
		`608 554 0:42 / / rw,relatime master:289 - overlay overlay rw,lowerdir=/var/lib/docker/overlay2/l/RQH52YWGJKBXL6THNS7EASIUNY:/var/lib/docker/overlay2/l/EHJME4ZW2BP2W7SGOCNTKY76NI,upperdir=/var/lib/docker/overlay2/241a77e9a6a048e54d5b5700afaeab6071cb5ffef1fd2acbebbf19935a429897/diff,workdir=/var/lib/docker/overlay2/241a77e9a6a048e54d5b5700afaeab6071cb5ffef1fd2acbebbf19935a429897/work
609 608 0:46 / /proc rw,nosuid,nodev,noexec,relatime - proc proc rw
610 608 0:47 / /dev rw,nosuid - tmpfs tmpfs rw,size=65536k,mode=755,inode64
611 610 0:48 / /dev/pts rw,nosuid,noexec,relatime - devpts devpts rw,gid=5,mode=620,ptmxmode=666
615 608 0:49 / /sys ro,nosuid,nodev,noexec,relatime - sysfs sysfs ro
623 615 0:28 / /sys/fs/cgroup ro,nosuid,nodev,noexec,relatime - cgroup2 cgroup rw,nsdelegate,memory_recursiveprot
624 610 0:45 / /dev/mqueue rw,nosuid,nodev,noexec,relatime - mqueue mqueue rw
625 610 0:50 / /dev/shm rw,nosuid,nodev,noexec,relatime - tmpfs shm rw,size=65536k,inode64
626 608 259:1 /var/lib/docker/containers/0cfa82bf3ab29da271548d6a044e95c948c6fd2f7578fb41833a44ca23da425f/resolv.conf /etc/resolv.conf rw,relatime - ext4 /dev/root rw,discard,errors=remount-ro
627 608 259:1 /var/lib/docker/containers/0cfa82bf3ab29da271548d6a044e95c948c6fd2f7578fb41833a44ca23da425f/hostname /etc/hostname rw,relatime - ext4 /dev/root rw,discard,errors=remount-ro
628 608 259:1 /var/lib/docker/containers/0cfa82bf3ab29da271548d6a044e95c948c6fd2f7578fb41833a44ca23da425f/hosts /etc/hosts rw,relatime - ext4 /dev/root rw,discard,errors=remount-ro
555 609 0:46 /bus /proc/bus ro,nosuid,nodev,noexec,relatime - proc proc rw
556 609 0:46 /fs /proc/fs ro,nosuid,nodev,noexec,relatime - proc proc rw
557 609 0:46 /irq /proc/irq ro,nosuid,nodev,noexec,relatime - proc proc rw
558 609 0:46 /sys /proc/sys ro,nosuid,nodev,noexec,relatime - proc proc rw
559 609 0:46 /sysrq-trigger /proc/sysrq-trigger ro,nosuid,nodev,noexec,relatime - proc proc rw
560 609 0:51 / /proc/acpi ro,relatime - tmpfs tmpfs ro,inode64
561 609 0:47 /null /proc/kcore rw,nosuid - tmpfs tmpfs rw,size=65536k,mode=755,inode64
562 609 0:47 /null /proc/keys rw,nosuid - tmpfs tmpfs rw,size=65536k,mode=755,inode64
563 609 0:47 /null /proc/timer_list rw,nosuid - tmpfs tmpfs rw,size=65536k,mode=755,inode64
564 609 0:52 / /proc/scsi ro,relatime - tmpfs tmpfs ro,inode64
565 615 0:53 / /sys/firmware ro,relatime - tmpfs tmpfs ro,inode64
`: "0cfa82bf3ab29da271548d6a044e95c948c6fd2f7578fb41833a44ca23da425f",
		`2775 2588 0:310 / / rw,relatime master:760 - overlay overlay rw,lowerdir=/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/163/fs,upperdir=/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/164/fs,workdir=/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/164/work
2776 2775 0:312 / /proc rw,nosuid,nodev,noexec,relatime - proc proc rw
2777 2775 0:313 / /dev rw,nosuid - tmpfs tmpfs rw,size=65536k,mode=755
2778 2777 0:314 / /dev/pts rw,nosuid,noexec,relatime - devpts devpts rw,gid=5,mode=620,ptmxmode=666
2779 2777 0:301 / /dev/mqueue rw,nosuid,nodev,noexec,relatime - mqueue mqueue rw
2780 2775 0:306 / /sys ro,nosuid,nodev,noexec,relatime - sysfs sysfs ro
2781 2780 0:24 / /sys/fs/cgroup ro,nosuid,nodev,noexec,relatime - cgroup2 cgroup rw
2782 2775 8:1 /var/lib/kubelet/pods/e50f2933-b36b-4e3b-89b1-280707f7bc53/etc-hosts /etc/hosts rw,relatime - ext4 /dev/sda1 rw,commit=30
2783 2777 8:1 /var/lib/kubelet/pods/e50f2933-b36b-4e3b-89b1-280707f7bc53/containers/bash2/3404dd90 /dev/termination-log rw,relatime - ext4 /dev/sda1 rw,commit=30
2784 2775 8:1 /var/lib/containerd/io.containerd.grpc.v1.cri/sandboxes/c5866f3bfef020a8c045540b4a05dd9d193443740be76437a6130c7759ab2076/hostname /etc/hostname rw,nosuid,nodev,relatime - ext4 /dev/sda1 rw,commit=30
2785 2775 8:1 /var/lib/containerd/io.containerd.grpc.v1.cri/sandboxes/c5866f3bfef020a8c045540b4a05dd9d193443740be76437a6130c7759ab2076/resolv.conf /etc/resolv.conf rw,nosuid,nodev,relatime - ext4 /dev/sda1 rw,commit=30
2786 2777 0:298 / /dev/shm rw,nosuid,nodev,noexec,relatime - tmpfs shm rw,size=65536k
2787 2775 0:297 / /var/run/secrets/kubernetes.io/serviceaccount ro,relatime - tmpfs tmpfs rw,size=2880088k
2602 2777 0:314 /0 /dev/console rw,nosuid,noexec,relatime - devpts devpts rw,gid=5,mode=620,ptmxmode=666
2603 2776 0:312 /bus /proc/bus ro,nosuid,nodev,noexec,relatime - proc proc rw
2604 2776 0:312 /fs /proc/fs ro,nosuid,nodev,noexec,relatime - proc proc rw
2605 2776 0:312 /irq /proc/irq ro,nosuid,nodev,noexec,relatime - proc proc rw
2606 2776 0:312 /sys /proc/sys ro,nosuid,nodev,noexec,relatime - proc proc rw
2607 2776 0:312 /sysrq-trigger /proc/sysrq-trigger ro,nosuid,nodev,noexec,relatime - proc proc rw
2608 2776 0:315 / /proc/acpi ro,relatime - tmpfs tmpfs ro
2609 2776 0:313 /null /proc/kcore rw,nosuid - tmpfs tmpfs rw,size=65536k,mode=755
2610 2776 0:313 /null /proc/keys rw,nosuid - tmpfs tmpfs rw,size=65536k,mode=755
2611 2776 0:313 /null /proc/timer_list rw,nosuid - tmpfs tmpfs rw,size=65536k,mode=755
2612 2776 0:316 / /proc/scsi ro,relatime - tmpfs tmpfs ro
2613 2780 0:317 / /sys/firmware ro,relatime - tmpfs tmpfs ro
`: "",
		`2208 2025 0:249 / / rw,relatime master:691 - overlay overlay rw,lowerdir=/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/133/fs,upperdir=/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/134/fs,workdir=/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/134/work
2209 2208 0:251 / /proc rw,nosuid,nodev,noexec,relatime - proc proc rw
2210 2208 0:252 / /dev rw,nosuid - tmpfs tmpfs rw,size=65536k,mode=755
2211 2210 0:253 / /dev/pts rw,nosuid,noexec,relatime - devpts devpts rw,gid=5,mode=620,ptmxmode=666
2212 2210 0:91 / /dev/mqueue rw,nosuid,nodev,noexec,relatime - mqueue mqueue rw
2213 2208 0:96 / /sys ro,nosuid,nodev,noexec,relatime - sysfs sysfs ro
2214 2213 0:24 / /sys/fs/cgroup ro,nosuid,nodev,noexec,relatime - cgroup2 cgroup rw
2215 2208 8:1 /var/lib/kubelet/pods/ce225bf3-5dbb-44d7-9591-29d1cc65cdc6/volumes/kubernetes.io~empty-dir/tmpdir /tmp rw,relatime - ext4 /dev/sda1 rw,commit=30
2216 2210 0:88 / /dev/shm rw,nosuid,nodev,noexec,relatime - tmpfs shm rw,size=65536k
2217 2210 8:1 /var/lib/kubelet/pods/ce225bf3-5dbb-44d7-9591-29d1cc65cdc6/containers/agent/22381dc2 /dev/termination-log rw,relatime - ext4 /dev/sda1 rw,commit=30
2218 2208 8:1 /var/lib/kubelet/pods/ce225bf3-5dbb-44d7-9591-29d1cc65cdc6/etc-hosts /etc/hosts rw,relatime - ext4 /dev/sda1 rw,commit=30
2219 2208 8:1 /var/lib/kubelet/pods/ce225bf3-5dbb-44d7-9591-29d1cc65cdc6/volumes/kubernetes.io~empty-dir/config /etc/datadog-agent rw,relatime - ext4 /dev/sda1 rw,commit=30
2220 2208 8:1 /var/lib/containerd/io.containerd.grpc.v1.cri/sandboxes/c0a82a3506b0366c9666f6dbe71c783abeb26ba65e312e918a49e10a277196d0/hostname /etc/hostname rw,nosuid,nodev,relatime - ext4 /dev/sda1 rw,commit=30
2221 2208 8:1 /var/lib/containerd/io.containerd.grpc.v1.cri/sandboxes/c0a82a3506b0366c9666f6dbe71c783abeb26ba65e312e918a49e10a277196d0/resolv.conf /etc/resolv.conf rw,nosuid,nodev,relatime - ext4 /dev/sda1 rw,commit=30
2222 2208 0:19 / /host/proc ro,relatime - proc proc rw
2223 2222 0:27 / /host/proc/sys/fs/binfmt_misc rw,relatime - autofs systemd-1 rw,fd=30,pgrp=0,timeout=0,minproto=5,maxproto=5,direct,pipe_ino=1284
2224 2223 0:45 / /host/proc/sys/fs/binfmt_misc rw,nosuid,nodev,noexec,relatime - binfmt_misc binfmt_misc rw
2225 2219 8:1 /var/lib/kubelet/pods/ce225bf3-5dbb-44d7-9591-29d1cc65cdc6/volumes/kubernetes.io~configmap/installinfo/..2022_10_25_08_56_17.196723275/install_info /etc/datadog-agent/install_info ro,relatime - ext4 /dev/sda1 rw,commit=30
2226 2208 8:1 /var/lib/datadog-agent/logs /opt/datadog-agent/run rw,nosuid,nodev,noexec,relatime - ext4 /dev/sda1 rw,commit=30
2227 2208 8:1 /var/log/pods /var/log/pods ro,relatime - ext4 /dev/sda1 rw,commit=30
2228 2208 8:1 /var/log/containers /var/log/containers ro,relatime - ext4 /dev/sda1 rw,commit=30
2229 2208 0:23 /datadog /var/run/datadog rw,nosuid,nodev - tmpfs tmpfs rw,size=805192k,nr_inodes=819200,mode=755
2230 2208 0:23 / /host/var/run ro - tmpfs tmpfs rw,size=805192k,nr_inodes=819200,mode=755
2231 2230 0:42 / /host/var/run/containerd/io.containerd.grpc.v1.cri/sandboxes/188cedbba84c09ef9084420db548c00fadeb590e6dcd955cad3413f8339bce84/shm rw,nosuid,nodev,noexec,relatime - tmpfs shm rw,size=65536k
2232 2230 0:44 / /host/var/run/containerd/io.containerd.runtime.v2.task/k8s.io/188cedbba84c09ef9084420db548c00fadeb590e6dcd955cad3413f8339bce84/rootfs rw,relatime - overlay overlay rw,lowerdir=/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/42/fs,upperdir=/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/43/fs,workdir=/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/43/work
2233 2230 0:56 / /host/var/run/containerd/io.containerd.runtime.v2.task/k8s.io/f0d7bcb99d31a341fbcc1c17e7a91c223a16997168c4ed756f61448de95ec0ab/rootfs rw,relatime - overlay overlay rw,lowerdir=/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/3/fs:/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/2/fs:/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/1/fs,upperdir=/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/44/fs,workdir=/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/44/work
2234 2230 0:63 / /host/var/run/containerd/io.containerd.grpc.v1.cri/sandboxes/67d04649323de52cee959f9cd3172469cc29b914569bf1b33761ba75b344b2e0/shm rw,nosuid,nodev,noexec,relatime - tmpfs shm rw,size=65536k
2235 2230 0:64 / /host/var/run/containerd/io.containerd.runtime.v2.task/k8s.io/67d04649323de52cee959f9cd3172469cc29b914569bf1b33761ba75b344b2e0/rootfs rw,relatime - overlay overlay rw,lowerdir=/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/42/fs,upperdir=/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/45/fs,workdir=/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/45/work
2236 2230 0:74 / /host/var/run/containerd/io.containerd.grpc.v1.cri/sandboxes/82c2fc7d2c974f108c8ed035393ba4864881fca51d34d68ab65adf9b3ecb1e39/shm rw,nosuid,nodev,noexec,relatime - tmpfs shm rw,size=65536k
2237 2230 0:75 / /host/var/run/containerd/io.containerd.runtime.v2.task/k8s.io/82c2fc7d2c974f108c8ed035393ba4864881fca51d34d68ab65adf9b3ecb1e39/rootfs rw,relatime - overlay overlay rw,lowerdir=/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/42/fs,upperdir=/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/46/fs,workdir=/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/46/work
2238 2230 0:4 net:[4026532317] /host/var/run/netns/cni-a6d26b34-6006-776c-6297-d43cbd377e1a rw - nsfs nsfs rw
2239 2230 0:88 / /host/var/run/containerd/io.containerd.grpc.v1.cri/sandboxes/c0a82a3506b0366c9666f6dbe71c783abeb26ba65e312e918a49e10a277196d0/shm rw,nosuid,nodev,noexec,relatime - tmpfs shm rw,size=65536k
2240 2230 0:89 / /host/var/run/containerd/io.containerd.runtime.v2.task/k8s.io/c0a82a3506b0366c9666f6dbe71c783abeb26ba65e312e918a49e10a277196d0/rootfs rw,relatime - overlay overlay rw,lowerdir=/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/42/fs,upperdir=/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/47/fs,workdir=/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/47/work
2241 2230 0:4 net:[4026532392] /host/var/run/netns/cni-b9ddcb41-083c-d886-cc1b-ad856fae244a rw - nsfs nsfs rw
2242 2230 0:100 / /host/var/run/containerd/io.containerd.grpc.v1.cri/sandboxes/006c472d636bbaa9bce73084d7545085d4b6127f8e6ffc39245ebc1f65284f1a/shm rw,nosuid,nodev,noexec,relatime - tmpfs shm rw,size=65536k
2243 2230 0:101 / /host/var/run/containerd/io.containerd.runtime.v2.task/k8s.io/006c472d636bbaa9bce73084d7545085d4b6127f8e6ffc39245ebc1f65284f1a/rootfs rw,relatime - overlay overlay rw,lowerdir=/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/42/fs,upperdir=/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/48/fs,workdir=/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/48/work
2244 2230 0:112 / /host/var/run/containerd/io.containerd.runtime.v2.task/k8s.io/1c2200d42c9b200327a9f369b680ef0fabd20ae1fd322abb5f37e480037c40fe/rootfs rw,relatime - overlay overlay rw,lowerdir=/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/51/fs:/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/50/fs:/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/49/fs,upperdir=/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/52/fs,workdir=/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/52/work
2245 2230 0:120 / /host/var/run/containerd/io.containerd.runtime.v2.task/k8s.io/3aaa22321201b33a7630c48471a3caa31b14e2d71011333751f174fb44b16944/rootfs rw,relatime - overlay overlay rw,lowerdir=/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/91/fs:/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/90/fs:/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/89/fs:/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/88/fs:/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/87/fs:/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/86/fs:/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/85/fs:/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/84/fs:/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/83/fs:/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/82/fs:/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/81/fs:/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/80/fs:/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/79/fs:/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/78/fs:/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/77/fs:/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/76/fs:/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/75/fs:/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/74/fs:/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/73/fs:/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/72/fs:/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/71/fs:/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/70/fs:/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/69/fs:/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/68/fs:/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/67/fs:/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/66/fs:/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/65/fs:/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/64/fs:/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/63/fs:/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/62/fs:/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/61/fs:/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/60/fs:/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/59/fs:/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/58/fs:/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/57/fs:/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/56/fs:/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/55/fs:/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/54/fs:/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/53/fs,upperdir=/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/92/fs,workdir=/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/92/work
2246 2230 0:128 / /host/var/run/containerd/io.containerd.runtime.v2.task/k8s.io/feecdc0737367e535ad3111be3570aa9d8f3efab4db569a9c93dff3b82fb749b/rootfs rw,relatime - overlay overlay rw,lowerdir=/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/97/fs:/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/96/fs:/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/95/fs:/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/94/fs:/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/93/fs,upperdir=/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/98/fs,workdir=/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/98/work
2247 2230 0:4 net:[4026532475] /host/var/run/netns/cni-e418c801-72a5-c80e-541c-5b7a706f8180 rw - nsfs nsfs rw
2248 2230 0:4 net:[4026532543] /host/var/run/netns/cni-d616515e-36c5-4fd5-3de5-3e0d01ca1299 rw - nsfs nsfs rw
2249 2230 0:4 net:[4026532615] /host/var/run/netns/cni-01c3425d-10c5-ecd0-b7f0-4dcb9b514038 rw - nsfs nsfs rw
2250 2230 0:139 / /host/var/run/containerd/io.containerd.grpc.v1.cri/sandboxes/71ff5735b57177a4f24269bee3e4961a248ecf495c1ff63fa8f95c74bc9206b4/shm rw,nosuid,nodev,noexec,relatime - tmpfs shm rw,size=65536k
2251 2230 0:140 / /host/var/run/containerd/io.containerd.grpc.v1.cri/sandboxes/5bbba3a7d2259ada29c791ad8ae1a5f5dc52f176792e92f78695bde2da6a6f63/shm rw,nosuid,nodev,noexec,relatime - tmpfs shm rw,size=65536k
2252 2230 0:141 / /host/var/run/containerd/io.containerd.grpc.v1.cri/sandboxes/a16f90bbb1340ea0fc60e4c94b2adca5acb7a2d191020de17b99800d9942aec5/shm rw,nosuid,nodev,noexec,relatime - tmpfs shm rw,size=65536k
2253 2230 0:142 / /host/var/run/containerd/io.containerd.runtime.v2.task/k8s.io/5bbba3a7d2259ada29c791ad8ae1a5f5dc52f176792e92f78695bde2da6a6f63/rootfs rw,relatime - overlay overlay rw,lowerdir=/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/42/fs,upperdir=/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/101/fs,workdir=/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/101/work
2254 2230 0:144 / /host/var/run/containerd/io.containerd.runtime.v2.task/k8s.io/71ff5735b57177a4f24269bee3e4961a248ecf495c1ff63fa8f95c74bc9206b4/rootfs rw,relatime - overlay overlay rw,lowerdir=/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/42/fs,upperdir=/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/100/fs,workdir=/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/100/work
2255 2230 0:146 / /host/var/run/containerd/io.containerd.runtime.v2.task/k8s.io/a16f90bbb1340ea0fc60e4c94b2adca5acb7a2d191020de17b99800d9942aec5/rootfs rw,relatime - overlay overlay rw,lowerdir=/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/42/fs,upperdir=/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/102/fs,workdir=/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/102/work
2256 2230 0:4 net:[4026532767] /host/var/run/netns/cni-635fdd5c-891b-7dce-6359-3f22711c901f rw - nsfs nsfs rw
2257 2230 0:180 / /host/var/run/containerd/io.containerd.grpc.v1.cri/sandboxes/3380ce37d9657a68c2be7e2d1a5f3a7c6b6434d98c512650efa81489008e16e8/shm rw,nosuid,nodev,noexec,relatime - tmpfs shm rw,size=65536k
2258 2230 0:183 / /host/var/run/containerd/io.containerd.runtime.v2.task/k8s.io/3380ce37d9657a68c2be7e2d1a5f3a7c6b6434d98c512650efa81489008e16e8/rootfs rw,relatime - overlay overlay rw,lowerdir=/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/42/fs,upperdir=/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/104/fs,workdir=/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/104/work
2259 2230 0:4 net:[4026532845] /host/var/run/netns/cni-caed5c60-e9fd-55e4-fd26-9d3522d7eeda rw - nsfs nsfs rw
2260 2230 0:203 / /host/var/run/containerd/io.containerd.grpc.v1.cri/sandboxes/4e0303bc6d20caffed804b11c0d01dd2c8c4e9309d8ef632757b009a58aa089b/shm rw,nosuid,nodev,noexec,relatime - tmpfs shm rw,size=65536k
2261 2230 0:204 / /host/var/run/containerd/io.containerd.runtime.v2.task/k8s.io/4e0303bc6d20caffed804b11c0d01dd2c8c4e9309d8ef632757b009a58aa089b/rootfs rw,relatime - overlay overlay rw,lowerdir=/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/42/fs,upperdir=/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/105/fs,workdir=/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/105/work
2262 2230 0:223 / /host/var/run/containerd/io.containerd.runtime.v2.task/k8s.io/d89d74b92f8548ad9b8ac4299736b6f4f4665dee36c392fb6956acb956ede47d/rootfs rw,relatime - overlay overlay rw,lowerdir=/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/20/fs:/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/19/fs:/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/18/fs:/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/17/fs:/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/16/fs:/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/4/fs,upperdir=/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/107/fs,workdir=/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/107/work
2263 2230 0:148 / /host/var/run/containerd/io.containerd.runtime.v2.task/k8s.io/4a9b64ac877dbf02470a508c34bdc48d9aa3377c2f4ed33f66fb66c34d8421d4/rootfs rw,relatime - overlay overlay rw,lowerdir=/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/117/fs:/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/116/fs:/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/115/fs:/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/114/fs:/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/113/fs:/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/112/fs:/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/111/fs:/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/109/fs,upperdir=/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/118/fs,workdir=/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/118/work
2264 2230 0:188 / /host/var/run/containerd/io.containerd.runtime.v2.task/k8s.io/876c755819a777a720e6683114bbd0372bca83e7a547e401b63ae1a7cb754b79/rootfs rw,relatime - overlay overlay rw,lowerdir=/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/124/fs:/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/123/fs:/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/122/fs:/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/121/fs:/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/120/fs:/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/119/fs,upperdir=/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/125/fs,workdir=/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/125/work
2265 2230 0:217 / /host/var/run/containerd/io.containerd.runtime.v2.task/k8s.io/faf62a9d55405c0ca0e9becbd6e02b22ecfab2c8f466294d188e6a3d34299032/rootfs rw,relatime - overlay overlay rw,lowerdir=/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/99/fs,upperdir=/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/129/fs,workdir=/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/129/work
2266 2230 0:233 / /host/var/run/containerd/io.containerd.runtime.v2.task/k8s.io/79fc04c9a48dae0de5ac951860bbf811d660f67a96c84d62162cc10972579d3f/rootfs rw,relatime - overlay overlay rw,lowerdir=/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/128/fs,upperdir=/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/130/fs,workdir=/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/130/work
2267 2230 0:241 / /host/var/run/containerd/io.containerd.runtime.v2.task/k8s.io/84952b0516872d00333667f6f132dcacb1a935f0d19862b54c5e045d45f4c642/rootfs rw,relatime - overlay overlay rw,lowerdir=/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/131/fs,upperdir=/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/132/fs,workdir=/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/132/work
2268 2230 0:249 / /host/var/run/containerd/io.containerd.runtime.v2.task/k8s.io/fc7038bc73a8d3850c66ddbfb0b2901afa378bfcbb942cc384b051767e4ac6b0/rootfs rw,relatime - overlay overlay rw,lowerdir=/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/133/fs,upperdir=/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/134/fs,workdir=/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/134/work
2269 2268 0:249 / /host/var/run/containerd/io.containerd.runtime.v2.task/k8s.io/fc7038bc73a8d3850c66ddbfb0b2901afa378bfcbb942cc384b051767e4ac6b0/rootfs rw,relatime - overlay overlay rw,lowerdir=/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/133/fs,upperdir=/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/134/fs,workdir=/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/134/work
2270 2269 0:251 / /host/var/run/containerd/io.containerd.runtime.v2.task/k8s.io/fc7038bc73a8d3850c66ddbfb0b2901afa378bfcbb942cc384b051767e4ac6b0/rootfs/proc rw,nosuid,nodev,noexec,relatime - proc proc rw
2271 2269 0:252 / /host/var/run/containerd/io.containerd.runtime.v2.task/k8s.io/fc7038bc73a8d3850c66ddbfb0b2901afa378bfcbb942cc384b051767e4ac6b0/rootfs/dev rw,nosuid - tmpfs tmpfs rw,size=65536k,mode=755
2272 2271 0:253 / /host/var/run/containerd/io.containerd.runtime.v2.task/k8s.io/fc7038bc73a8d3850c66ddbfb0b2901afa378bfcbb942cc384b051767e4ac6b0/rootfs/dev/pts rw,nosuid,noexec,relatime - devpts devpts rw,gid=5,mode=620,ptmxmode=666
2273 2271 0:91 / /host/var/run/containerd/io.containerd.runtime.v2.task/k8s.io/fc7038bc73a8d3850c66ddbfb0b2901afa378bfcbb942cc384b051767e4ac6b0/rootfs/dev/mqueue rw,nosuid,nodev,noexec,relatime - mqueue mqueue rw
2274 2271 0:88 / /host/var/run/containerd/io.containerd.runtime.v2.task/k8s.io/fc7038bc73a8d3850c66ddbfb0b2901afa378bfcbb942cc384b051767e4ac6b0/rootfs/dev/shm rw,nosuid,nodev,noexec,relatime - tmpfs shm rw,size=65536k
2275 2271 8:1 /var/lib/kubelet/pods/ce225bf3-5dbb-44d7-9591-29d1cc65cdc6/containers/agent/22381dc2 /host/var/run/containerd/io.containerd.runtime.v2.task/k8s.io/fc7038bc73a8d3850c66ddbfb0b2901afa378bfcbb942cc384b051767e4ac6b0/rootfs/dev/termination-log rw,relatime - ext4 /dev/sda1 rw,commit=30
2276 2269 0:96 / /host/var/run/containerd/io.containerd.runtime.v2.task/k8s.io/fc7038bc73a8d3850c66ddbfb0b2901afa378bfcbb942cc384b051767e4ac6b0/rootfs/sys ro,nosuid,nodev,noexec,relatime - sysfs sysfs ro
2277 2276 0:24 / /host/var/run/containerd/io.containerd.runtime.v2.task/k8s.io/fc7038bc73a8d3850c66ddbfb0b2901afa378bfcbb942cc384b051767e4ac6b0/rootfs/sys/fs/cgroup ro,nosuid,nodev,noexec,relatime - cgroup2 cgroup rw
2278 2269 8:1 /var/lib/kubelet/pods/ce225bf3-5dbb-44d7-9591-29d1cc65cdc6/volumes/kubernetes.io~empty-dir/tmpdir /host/var/run/containerd/io.containerd.runtime.v2.task/k8s.io/fc7038bc73a8d3850c66ddbfb0b2901afa378bfcbb942cc384b051767e4ac6b0/rootfs/tmp rw,relatime - ext4 /dev/sda1 rw,commit=30
2279 2269 8:1 /var/lib/kubelet/pods/ce225bf3-5dbb-44d7-9591-29d1cc65cdc6/etc-hosts /host/var/run/containerd/io.containerd.runtime.v2.task/k8s.io/fc7038bc73a8d3850c66ddbfb0b2901afa378bfcbb942cc384b051767e4ac6b0/rootfs/etc/hosts rw,relatime - ext4 /dev/sda1 rw,commit=30
2280 2269 8:1 /var/lib/kubelet/pods/ce225bf3-5dbb-44d7-9591-29d1cc65cdc6/volumes/kubernetes.io~empty-dir/config /host/var/run/containerd/io.containerd.runtime.v2.task/k8s.io/fc7038bc73a8d3850c66ddbfb0b2901afa378bfcbb942cc384b051767e4ac6b0/rootfs/etc/datadog-agent rw,relatime - ext4 /dev/sda1 rw,commit=30
2281 2280 8:1 /var/lib/kubelet/pods/ce225bf3-5dbb-44d7-9591-29d1cc65cdc6/volumes/kubernetes.io~configmap/installinfo/..2022_10_25_08_56_17.196723275/install_info /host/var/run/containerd/io.containerd.runtime.v2.task/k8s.io/fc7038bc73a8d3850c66ddbfb0b2901afa378bfcbb942cc384b051767e4ac6b0/rootfs/etc/datadog-agent/install_info ro,relatime - ext4 /dev/sda1 rw,commit=30
2282 2269 8:1 /var/lib/containerd/io.containerd.grpc.v1.cri/sandboxes/c0a82a3506b0366c9666f6dbe71c783abeb26ba65e312e918a49e10a277196d0/hostname /host/var/run/containerd/io.containerd.runtime.v2.task/k8s.io/fc7038bc73a8d3850c66ddbfb0b2901afa378bfcbb942cc384b051767e4ac6b0/rootfs/etc/hostname rw,nosuid,nodev,relatime - ext4 /dev/sda1 rw,commit=30
2283 2269 8:1 /var/lib/containerd/io.containerd.grpc.v1.cri/sandboxes/c0a82a3506b0366c9666f6dbe71c783abeb26ba65e312e918a49e10a277196d0/resolv.conf /host/var/run/containerd/io.containerd.runtime.v2.task/k8s.io/fc7038bc73a8d3850c66ddbfb0b2901afa378bfcbb942cc384b051767e4ac6b0/rootfs/etc/resolv.conf rw,nosuid,nodev,relatime - ext4 /dev/sda1 rw,commit=30
2284 2269 0:19 / /host/var/run/containerd/io.containerd.runtime.v2.task/k8s.io/fc7038bc73a8d3850c66ddbfb0b2901afa378bfcbb942cc384b051767e4ac6b0/rootfs/host/proc ro,relatime - proc proc rw
2285 2284 0:27 / /host/var/run/containerd/io.containerd.runtime.v2.task/k8s.io/fc7038bc73a8d3850c66ddbfb0b2901afa378bfcbb942cc384b051767e4ac6b0/rootfs/host/proc/sys/fs/binfmt_misc rw,relatime - autofs systemd-1 rw,fd=30,pgrp=0,timeout=0,minproto=5,maxproto=5,direct,pipe_ino=1284
2286 2285 0:45 / /host/var/run/containerd/io.containerd.runtime.v2.task/k8s.io/fc7038bc73a8d3850c66ddbfb0b2901afa378bfcbb942cc384b051767e4ac6b0/rootfs/host/proc/sys/fs/binfmt_misc rw,nosuid,nodev,noexec,relatime - binfmt_misc binfmt_misc rw
2287 2269 8:1 /var/lib/datadog-agent/logs /host/var/run/containerd/io.containerd.runtime.v2.task/k8s.io/fc7038bc73a8d3850c66ddbfb0b2901afa378bfcbb942cc384b051767e4ac6b0/rootfs/opt/datadog-agent/run rw,nosuid,nodev,noexec,relatime - ext4 /dev/sda1 rw,commit=30
2288 2269 8:1 /var/log/pods /host/var/run/containerd/io.containerd.runtime.v2.task/k8s.io/fc7038bc73a8d3850c66ddbfb0b2901afa378bfcbb942cc384b051767e4ac6b0/rootfs/var/log/pods ro,relatime - ext4 /dev/sda1 rw,commit=30
2289 2269 8:1 /var/log/containers /host/var/run/containerd/io.containerd.runtime.v2.task/k8s.io/fc7038bc73a8d3850c66ddbfb0b2901afa378bfcbb942cc384b051767e4ac6b0/rootfs/var/log/containers ro,relatime - ext4 /dev/sda1 rw,commit=30
2290 2269 0:23 /datadog /host/var/run/containerd/io.containerd.runtime.v2.task/k8s.io/fc7038bc73a8d3850c66ddbfb0b2901afa378bfcbb942cc384b051767e4ac6b0/rootfs/var/run/datadog rw,nosuid,nodev - tmpfs tmpfs rw,size=805192k,nr_inodes=819200,mode=755
2291 2208 0:34 /os-release /host/etc/os-release ro,relatime - overlay overlayfs rw,lowerdir=/etc,upperdir=/tmp/etc_overlay/etc,workdir=/tmp/etc_overlay/.work
2292 2208 8:1 /var/lib/kubelet/pods/ce225bf3-5dbb-44d7-9591-29d1cc65cdc6/volumes/kubernetes.io~empty-dir/logdatadog /var/log/datadog rw,relatime - ext4 /dev/sda1 rw,commit=30
2293 2208 8:1 /var/lib/containerd/io.containerd.grpc.v1.cri/containers/fc7038bc73a8d3850c66ddbfb0b2901afa378bfcbb942cc384b051767e4ac6b0/volumes/33a073eeb8406d18d9007e3dcf8690235726a4f5aa81dd293650f75fc2c2a620 /var/run/s6 rw,nosuid,nodev,relatime - ext4 /dev/sda1 rw,commit=30
2294 2208 8:1 /var/lib/docker/containers /var/lib/docker/containers ro,relatime - ext4 /dev/sda1 rw,commit=30
2295 2208 0:24 /../../../.. /host/sys/fs/cgroup ro,relatime - cgroup2 cgroup2 rw
2296 2208 0:85 / /var/run/secrets/kubernetes.io/serviceaccount ro,relatime - tmpfs tmpfs rw,size=2880088k
2026 2209 0:251 /bus /proc/bus ro,nosuid,nodev,noexec,relatime - proc proc rw
2027 2209 0:251 /fs /proc/fs ro,nosuid,nodev,noexec,relatime - proc proc rw
2028 2209 0:251 /irq /proc/irq ro,nosuid,nodev,noexec,relatime - proc proc rw
2029 2209 0:251 /sys /proc/sys ro,nosuid,nodev,noexec,relatime - proc proc rw
2030 2209 0:251 /sysrq-trigger /proc/sysrq-trigger ro,nosuid,nodev,noexec,relatime - proc proc rw
2031 2209 0:254 / /proc/acpi ro,relatime - tmpfs tmpfs ro
2032 2209 0:252 /null /proc/kcore rw,nosuid - tmpfs tmpfs rw,size=65536k,mode=755
2033 2209 0:252 /null /proc/keys rw,nosuid - tmpfs tmpfs rw,size=65536k,mode=755
2034 2209 0:252 /null /proc/timer_list rw,nosuid - tmpfs tmpfs rw,size=65536k,mode=755
2035 2209 0:255 / /proc/scsi ro,relatime - tmpfs tmpfs ro
2036 2213 0:256 / /sys/firmware ro,relatime - tmpfs tmpfs ro
`: "fc7038bc73a8d3850c66ddbfb0b2901afa378bfcbb942cc384b051767e4ac6b0",
		`1258 1249 254:1 /docker/volumes/0919c2d87ec8ba99f3c85fdada5fe26eca73b2fce73a5974d6030f30bf91cbaf/_data/lib/containerd/io.containerd.grpc.v1.cri/sandboxes/ca30bb64884083e29b1dc08a1081dd2df123f13f045dadb64dc346e56c0b6871/hostname /etc/hostname rw,relatime - ext4 /dev/vda1 rw,discard`: "",
	} {
		id := parseMountinfo(strings.NewReader(input))
		assert.Equal(t, expectedResult, id)
	}
}

func TestReadMountinfo(t *testing.T) {
	cid := "0cfa82bf3ab29da271548d6a044e95c948c6fd2f7578fb41833a44ca23da425f"
	mountinfoContents := fmt.Sprintf(`608 554 0:42 / / rw,relatime master:289 - overlay overlay rw,lowerdir=/var/lib/docker/overlay2/l/RQH52YWGJKBXL6THNS7EASIUNY:/var/lib/docker/overlay2/l/EHJME4ZW2BP2W7SGOCNTKY76NI,upperdir=/var/lib/docker/overlay2/241a77e9a6a048e54d5b5700afaeab6071cb5ffef1fd2acbebbf19935a429897/diff,workdir=/var/lib/docker/overlay2/241a77e9a6a048e54d5b5700afaeab6071cb5ffef1fd2acbebbf19935a429897/work
609 608 0:46 / /proc rw,nosuid,nodev,noexec,relatime - proc proc rw
610 608 0:47 / /dev rw,nosuid - tmpfs tmpfs rw,size=65536k,mode=755,inode64
611 610 0:48 / /dev/pts rw,nosuid,noexec,relatime - devpts devpts rw,gid=5,mode=620,ptmxmode=666
615 608 0:49 / /sys ro,nosuid,nodev,noexec,relatime - sysfs sysfs ro
623 615 0:28 / /sys/fs/cgroup ro,nosuid,nodev,noexec,relatime - cgroup2 cgroup rw,nsdelegate,memory_recursiveprot
624 610 0:45 / /dev/mqueue rw,nosuid,nodev,noexec,relatime - mqueue mqueue rw
625 610 0:50 / /dev/shm rw,nosuid,nodev,noexec,relatime - tmpfs shm rw,size=65536k,inode64
626 608 259:1 /var/lib/docker/containers/%[1]s/resolv.conf /etc/resolv.conf rw,relatime - ext4 /dev/root rw,discard,errors=remount-ro
627 608 259:1 /var/lib/docker/containers/%[1]s/hostname /etc/hostname rw,relatime - ext4 /dev/root rw,discard,errors=remount-ro
628 608 259:1 /var/lib/docker/containers/%[1]s/hosts /etc/hosts rw,relatime - ext4 /dev/root rw,discard,errors=remount-ro
555 609 0:46 /bus /proc/bus ro,nosuid,nodev,noexec,relatime - proc proc rw
556 609 0:46 /fs /proc/fs ro,nosuid,nodev,noexec,relatime - proc proc rw
557 609 0:46 /irq /proc/irq ro,nosuid,nodev,noexec,relatime - proc proc rw
558 609 0:46 /sys /proc/sys ro,nosuid,nodev,noexec,relatime - proc proc rw
559 609 0:46 /sysrq-trigger /proc/sysrq-trigger ro,nosuid,nodev,noexec,relatime - proc proc rw
560 609 0:51 / /proc/acpi ro,relatime - tmpfs tmpfs ro,inode64
561 609 0:47 /null /proc/kcore rw,nosuid - tmpfs tmpfs rw,size=65536k,mode=755,inode64
562 609 0:47 /null /proc/keys rw,nosuid - tmpfs tmpfs rw,size=65536k,mode=755,inode64
563 609 0:47 /null /proc/timer_list rw,nosuid - tmpfs tmpfs rw,size=65536k,mode=755,inode64
564 609 0:52 / /proc/scsi ro,relatime - tmpfs tmpfs ro,inode64
565 615 0:53 / /sys/firmware ro,relatime - tmpfs tmpfs ro,inode64
`, cid)

	tmpFile, err := ioutil.TempFile(os.TempDir(), "fake-mountinfo-")
	assert.NoError(t, err)

	defer os.Remove(tmpFile.Name())

	_, err = io.WriteString(tmpFile, mountinfoContents)
	assert.NoError(t, err)

	err = tmpFile.Close()
	assert.NoError(t, err)

	actualCID := readMountinfo(tmpFile.Name())
	assert.Equal(t, cid, actualCID)
}

func TestParsegroupControllerPath(t *testing.T) {
	// Test cases
	cases := []struct {
		name     string
		content  string
		expected map[string]string
	}{
		{
			name:     "cgroup2 normal case",
			content:  `0::/`,
			expected: map[string]string{"": "/"},
		},
		{
			name: "hybrid",
			content: `other_line
0::/
1:memory:/docker/abc123`,
			expected: map[string]string{
				"":       "/",
				"memory": "/docker/abc123",
			},
		},
		{
			name: "with other controllers",
			content: `other_line
12:pids:/docker/abc123
11:hugetlb:/docker/abc123
10:net_cls,net_prio:/docker/abc123
0::/docker/abc123
`,
			expected: map[string]string{
				"": "/docker/abc123",
			},
		},
		{
			name:     "no controller",
			content:  "empty",
			expected: map[string]string{},
		},
	}

	// Run test cases
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			reader := strings.NewReader(c.content)
			result := parseCgroupNodePath(reader)
			require.Equal(t, c.expected, result)
		})
	}
}

func TestGetCgroupInode(t *testing.T) {
	tests := []struct {
		description           string
		cgroupNodeDir         string
		procSelfCgroupContent string
		expectedResult        string
		controller            string
	}{
		{
			description:           "matching entry in /proc/self/cgroup and /proc/mounts - cgroup2 only",
			cgroupNodeDir:         "system.slice/docker-abcdef0123456789abcdef0123456789.scope",
			procSelfCgroupContent: "0::/system.slice/docker-abcdef0123456789abcdef0123456789.scope\n",
			expectedResult:        "in-%d", // Will be formatted with inode number
		},
		{
			description:   "matching entry in /proc/self/cgroup and /proc/mounts - cgroup/hybrid only",
			cgroupNodeDir: "system.slice/docker-abcdef0123456789abcdef0123456789.scope",
			procSelfCgroupContent: `
3:memory:/system.slice/docker-abcdef0123456789abcdef0123456789.scope
2:net_cls,net_prio:c
1:name=systemd:b
0::a
`,
			expectedResult: "in-%d",
			controller:     cgroupV1BaseController,
		},
		{
			description:   "non memory or empty controller",
			cgroupNodeDir: "system.slice/docker-abcdef0123456789abcdef0123456789.scope",
			procSelfCgroupContent: `
3:cpu:/system.slice/docker-abcdef0123456789abcdef0123456789.scope
2:net_cls,net_prio:c
1:name=systemd:b
0::a
`,
			expectedResult: "",
			controller:     "cpu",
		},
		{
			description:   "path does not exist",
			cgroupNodeDir: "dummy.scope",
			procSelfCgroupContent: `
3:memory:/system.slice/docker-abcdef0123456789abcdef0123456789.scope
2:net_cls,net_prio:c
1:name=systemd:b
0::a
`,
			expectedResult: "",
		},
		{
			description:           "no entry in /proc/self/cgroup",
			cgroupNodeDir:         "system.slice/docker-abcdef0123456789abcdef0123456789.scope",
			procSelfCgroupContent: "nothing",
			expectedResult:        "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			sysFsCgroupPath := path.Join(os.TempDir(), "sysfscgroup")
			groupControllerPath := path.Join(sysFsCgroupPath, tc.controller, tc.cgroupNodeDir)
			err := os.MkdirAll(groupControllerPath, 0755)
			require.NoError(t, err)
			defer os.RemoveAll(groupControllerPath)

			stat, err := os.Stat(groupControllerPath)
			require.NoError(t, err)
			expectedInode := ""
			if tc.expectedResult != "" {
				expectedInode = fmt.Sprintf(tc.expectedResult, stat.Sys().(*syscall.Stat_t).Ino)
			}

			procSelfCgroup, err := ioutil.TempFile("", "procselfcgroup")
			require.NoError(t, err)
			defer os.Remove(procSelfCgroup.Name())
			_, err = procSelfCgroup.WriteString(tc.procSelfCgroupContent)
			require.NoError(t, err)
			err = procSelfCgroup.Close()
			require.NoError(t, err)

			result := getCgroupInode(sysFsCgroupPath, procSelfCgroup.Name())
			require.Equal(t, expectedInode, result)
		})
	}
}

func TestReadCIDOrInode(t *testing.T) {
	tests := []struct {
		description           string
		procSelfCgroupContent string
		mountInfoContent      string
		isHostCgroupNs        bool
		expectedResult        string
		cgroupNodeDir         string
	}{
		{
			description:           "extract container-id from /proc/self/cgroup",
			procSelfCgroupContent: "4:blkio:/kubepods/burstable/podfd52ef25-a87d-11e9-9423-0800271a638e/8c046cb0b72cd4c99f51b5591cd5b095967f58ee003710a45280c28ee1a9c7fa\n",
			isHostCgroupNs:        true,
			expectedResult:        "8c046cb0b72cd4c99f51b5591cd5b095967f58ee003710a45280c28ee1a9c7fa", // Will be formatted with inode number
		},
		{
			description:           "extract container-id from /proc/self/cgroup in private cgroup ns",
			procSelfCgroupContent: "4:blkio:/kubepods/burstable/podfd52ef25-a87d-11e9-9423-0800271a638e/8c046cb0b72cd4c99f51b5591cd5b095967f58ee003710a45280c28ee1a9c7fa\n",
			expectedResult:        "8c046cb0b72cd4c99f51b5591cd5b095967f58ee003710a45280c28ee1a9c7fa", // Will be formatted with inode number
		},
		{
			description:      "extract container-id from mountinfo in private cgroup ns",
			mountInfoContent: "2282 2269 8:1 /var/lib/containerd/io.containerd.grpc.v1.cri/sandboxes/c0a82a3506b0366c9666f6dbe71c783abeb26ba65e312e918a49e10a277196d0/hostname /host/var/run/containerd/io.containerd.runtime.v2.task/k8s.io/fc7038bc73a8d3850c66ddbfb0b2901afa378bfcbb942cc384b051767e4ac6b0/rootfs/etc/hostname rw,nosuid,nodev,relatime - ext4 /dev/sda1 rw,commit=30\n",
			expectedResult:   "fc7038bc73a8d3850c66ddbfb0b2901afa378bfcbb942cc384b051767e4ac6b0",
		},
		{
			description:      "extract container-id from mountinfo",
			mountInfoContent: "2282 2269 8:1 /var/lib/containerd/io.containerd.grpc.v1.cri/sandboxes/c0a82a3506b0366c9666f6dbe71c783abeb26ba65e312e918a49e10a277196d0/hostname /host/var/run/containerd/io.containerd.runtime.v2.task/k8s.io/fc7038bc73a8d3850c66ddbfb0b2901afa378bfcbb942cc384b051767e4ac6b0/rootfs/etc/hostname rw,nosuid,nodev,relatime - ext4 /dev/sda1 rw,commit=30\n",
			expectedResult:   "fc7038bc73a8d3850c66ddbfb0b2901afa378bfcbb942cc384b051767e4ac6b0",
			isHostCgroupNs:   true,
		},
		{
			description:           "extract inode only in private cgroup ns",
			cgroupNodeDir:         "system.slice/docker-abcdef0123456789abcdef0123456789.scope",
			procSelfCgroupContent: "0::/system.slice/docker-abcdef0123456789abcdef0123456789.scope\n",
			expectedResult:        "in-%d",
		},
		{
			description:           "do not extract inode in host cgroup ns",
			cgroupNodeDir:         "system.slice/docker-abcdef0123456789abcdef0123456789.scope",
			procSelfCgroupContent: "0::/system.slice/docker-abcdef0123456789abcdef0123456789.scope\n",
			isHostCgroupNs:        true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			prevContainerID := containerID
			defer func() {
				containerID = prevContainerID
			}()

			sysFsCgroupPath := path.Join(os.TempDir(), "sysfscgroup")
			groupControllerPath := path.Join(sysFsCgroupPath, tc.cgroupNodeDir)
			err := os.MkdirAll(groupControllerPath, 0755)
			require.NoError(t, err)
			defer os.RemoveAll(groupControllerPath)

			stat, err := os.Stat(groupControllerPath)
			require.NoError(t, err)
			expectedResult := tc.expectedResult
			if strings.HasPrefix(tc.expectedResult, "in-") {
				expectedResult = fmt.Sprintf(tc.expectedResult, stat.Sys().(*syscall.Stat_t).Ino)
			}

			procSelfCgroup, err := ioutil.TempFile("", "procselfcgroup")
			require.NoError(t, err)
			defer os.Remove(procSelfCgroup.Name())
			_, err = procSelfCgroup.WriteString(tc.procSelfCgroupContent)
			require.NoError(t, err)
			err = procSelfCgroup.Close()
			require.NoError(t, err)

			mountInfo, err := ioutil.TempFile("", "mountInfo")
			require.NoError(t, err)
			defer os.Remove(mountInfo.Name())
			_, err = mountInfo.WriteString(tc.mountInfoContent)
			require.NoError(t, err)
			err = mountInfo.Close()
			require.NoError(t, err)

			readCIDOrInode("", procSelfCgroup.Name(), mountInfo.Name(), sysFsCgroupPath, true, tc.isHostCgroupNs)
			require.Equal(t, expectedResult, containerID)
		})
	}
}
