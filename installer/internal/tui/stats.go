package tui

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// sysSample is a raw counter snapshot from /proc, taken each tick. Rates are
// derived from the delta between two samples.
type sysSample struct {
	when       time.Time
	cpuTotal   uint64 // sum of all /proc/stat cpu fields
	cpuIdle    uint64 // idle + iowait
	memUsedPct float64
	netBytes   uint64 // rx+tx across non-loopback ifaces
	ioBytes    uint64 // (read+write sectors)*512 across real disks
	ok         bool
}

// sysStats is the computed, display-ready snapshot shown in the footer.
type sysStats struct {
	cpuPct  float64 // 0..100
	memPct  float64 // 0..100
	netBps  float64 // bytes/sec (rx+tx)
	ioBps   float64 // bytes/sec (read+write)
	haveNet bool
	haveIO  bool
}

// sampleSys reads /proc for the current counters. Best-effort: missing files
// just leave a field zero (ok stays true so partial stats still render).
func sampleSys(now time.Time) sysSample {
	s := sysSample{when: now, ok: true}

	if b, err := os.ReadFile("/proc/stat"); err == nil {
		for _, line := range strings.Split(string(b), "\n") {
			if strings.HasPrefix(line, "cpu ") {
				f := strings.Fields(line)[1:]
				var total, idle uint64
				for i, v := range f {
					n, _ := strconv.ParseUint(v, 10, 64)
					total += n
					if i == 3 || i == 4 { // idle, iowait
						idle += n
					}
				}
				s.cpuTotal, s.cpuIdle = total, idle
				break
			}
		}
	}

	if b, err := os.ReadFile("/proc/meminfo"); err == nil {
		var total, avail float64
		for _, line := range strings.Split(string(b), "\n") {
			f := strings.Fields(line)
			if len(f) < 2 {
				continue
			}
			v, _ := strconv.ParseFloat(f[1], 64)
			switch f[0] {
			case "MemTotal:":
				total = v
			case "MemAvailable:":
				avail = v
			}
		}
		if total > 0 {
			s.memUsedPct = (total - avail) / total * 100
		}
	}

	if b, err := os.ReadFile("/proc/net/dev"); err == nil {
		for _, line := range strings.Split(string(b), "\n") {
			i := strings.Index(line, ":")
			if i < 0 {
				continue
			}
			name := strings.TrimSpace(line[:i])
			if name == "lo" || name == "" {
				continue
			}
			f := strings.Fields(line[i+1:])
			if len(f) >= 9 {
				rx, _ := strconv.ParseUint(f[0], 10, 64)
				tx, _ := strconv.ParseUint(f[8], 10, 64)
				s.netBytes += rx + tx
			}
		}
	}

	if b, err := os.ReadFile("/proc/diskstats"); err == nil {
		for _, line := range strings.Split(string(b), "\n") {
			f := strings.Fields(line)
			if len(f) < 10 {
				continue
			}
			dev := f[2]
			// real disks only: skip partitions + loop/ram
			if strings.HasPrefix(dev, "loop") || strings.HasPrefix(dev, "ram") {
				continue
			}
			if len(dev) > 0 && dev[len(dev)-1] >= '0' && dev[len(dev)-1] <= '9' && !strings.HasPrefix(dev, "nvme") {
				continue // partition like sda1
			}
			rd, _ := strconv.ParseUint(f[5], 10, 64) // sectors read
			wr, _ := strconv.ParseUint(f[9], 10, 64) // sectors written
			s.ioBytes += (rd + wr) * 512
		}
	}
	return s
}

// computeStats derives display rates from two samples.
func computeStats(prev, cur sysSample) sysStats {
	var st sysStats
	st.memPct = cur.memUsedPct
	dt := cur.when.Sub(prev.when).Seconds()
	if !prev.ok || dt <= 0 {
		return st
	}
	if dTot := cur.cpuTotal - prev.cpuTotal; dTot > 0 {
		dIdle := cur.cpuIdle - prev.cpuIdle
		st.cpuPct = (1 - float64(dIdle)/float64(dTot)) * 100
	}
	if cur.netBytes >= prev.netBytes {
		st.netBps = float64(cur.netBytes-prev.netBytes) / dt
		st.haveNet = true
	}
	if cur.ioBytes >= prev.ioBytes {
		st.ioBps = float64(cur.ioBytes-prev.ioBytes) / dt
		st.haveIO = true
	}
	return st
}
