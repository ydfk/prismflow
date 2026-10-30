package hosttraffic

import (
	"bufio"
	"errors"
	"os"
	"strconv"
	"strings"
)

type Counters struct {
	Interface, BootID string
	Upload, Download  uint64
}

// Linux host 网络模式读取默认路由接口；不叠加虚拟网卡，以免同一数据重复计数。
func readCounters(device string) (Counters, error) {
	if device == "" {
		b, err := os.ReadFile("/proc/net/route")
		if err != nil {
			return Counters{}, errors.New("服务器网卡监控需要 Linux /proc")
		}
		for _, line := range strings.Split(string(b), "\n") {
			fields := strings.Fields(line)
			if len(fields) > 3 && fields[1] == "00000000" {
				flags, _ := strconv.ParseUint(fields[3], 16, 64)
				if flags&1 != 0 {
					device = fields[0]
					break
				}
			}
		}
	}
	if device == "" {
		return Counters{}, errors.New("未找到默认路由接口，请指定网卡")
	}
	file, err := os.Open("/proc/net/dev")
	if err != nil {
		return Counters{}, errors.New("无法读取服务器网卡计数")
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		name, values, ok := strings.Cut(scanner.Text(), ":")
		if !ok || strings.TrimSpace(name) != device {
			continue
		}
		fields := strings.Fields(values)
		if len(fields) < 9 {
			break
		}
		rx, e1 := strconv.ParseUint(fields[0], 10, 64)
		tx, e2 := strconv.ParseUint(fields[8], 10, 64)
		if e1 != nil || e2 != nil {
			break
		}
		boot, _ := os.ReadFile("/proc/sys/kernel/random/boot_id")
		return Counters{device, strings.TrimSpace(string(boot)), tx, rx}, nil
	}
	return Counters{}, errors.New("指定网卡不存在或计数无效")
}
