package statsd

import (
	"io"
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
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
