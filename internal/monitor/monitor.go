package monitor

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

type MetricsSample struct {
	Load   float64
	CPU    float64
	Mem    float64
	NetKB  float64
	OkLoad bool
	OkCPU  bool
	OkMem  bool
	OkNet  bool
}

type MetricHistory struct {
	Load []float64
	CPU  []float64
	Mem  []float64
	Net  []float64
}

type SystemInfo struct {
	Uptime string
	Disk   string
	Net    string
}

const (
	HistoryLength = 30
	unknownStr    = "unknown"
	loStr         = "lo"
	lo0Str        = "lo0"
)

func UpdateHistory(history MetricHistory, sample MetricsSample) MetricHistory {
	if sample.OkLoad {
		history.Load = append(history.Load, sample.Load)
		history.Load = trimHistory(history.Load, HistoryLength)
	}
	if sample.OkCPU {
		history.CPU = append(history.CPU, sample.CPU)
		history.CPU = trimHistory(history.CPU, HistoryLength)
	}
	if sample.OkMem {
		history.Mem = append(history.Mem, sample.Mem)
		history.Mem = trimHistory(history.Mem, HistoryLength)
	}
	if sample.OkNet {
		history.Net = append(history.Net, sample.NetKB)
		history.Net = trimHistory(history.Net, HistoryLength)
	}
	return history
}

func trimHistory(values []float64, maxLen int) []float64 {
	if len(values) <= maxLen {
		return values
	}
	return values[len(values)-maxLen:]
}

func SampleMetrics() MetricsSample {
	var sample MetricsSample
	if load, ok := getLoadAvg(); ok {
		sample.Load = load
		sample.OkLoad = true
	}
	if cpu, ok := getCPUUsage(); ok {
		sample.CPU = cpu
		sample.OkCPU = true
	}
	if mem, ok := getMemUsage(); ok {
		sample.Mem = mem
		sample.OkMem = true
	}
	if netKB, ok := getNetRateKB(); ok {
		sample.NetKB = netKB
		sample.OkNet = true
	}
	return sample
}

func SampleSystem() SystemInfo {
	var info SystemInfo
	info.Uptime = "UPTIME: " + getUptimeShort()

	if disk := getDiskSummary(); disk != "" {
		info.Disk = "DISK: " + disk
	}
	if net := getNetSummary(); net != "" {
		info.Net = "NET: " + net
	}
	return info
}

// Internal helper helpers

func runQuickCmd(cmd []string, timeout time.Duration) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	c := exec.CommandContext(ctx, cmd[0], cmd[1:]...)
	var out bytes.Buffer
	c.Stdout = &out
	c.Stderr = &out
	if err := c.Run(); err != nil {
		return "", err
	}
	return out.String(), nil
}

func parseFloat(s string) (float64, error) {
	return strconv.ParseFloat(strings.TrimSpace(s), 64)
}

func indexOf(fields []string, target string) int {
	for i, f := range fields {
		if f == target {
			return i
		}
	}
	return -1
}

func FormatRate(kbPerSec float64) string {
	if kbPerSec < 1024 {
		return fmt.Sprintf("%0.0fKB/s", kbPerSec)
	}
	return fmt.Sprintf("%0.1fMB/s", kbPerSec/1024.0)
}

// System logic

func getUptimeShort() string {
	if _, err := exec.LookPath("uptime"); err != nil {
		return unknownStr
	}
	out, err := runQuickCmd([]string{"uptime"}, 2*time.Second)
	if err != nil {
		return unknownStr
	}
	line := strings.TrimSpace(out)
	idx := strings.Index(line, " up ")
	if idx == -1 {
		return unknownStr
	}
	part := line[idx+4:]
	if cut := strings.Index(part, "load average"); cut != -1 {
		part = part[:cut]
	}
	if cut := strings.Index(part, "load averages"); cut != -1 {
		part = part[:cut]
	}
	if cut := strings.Index(part, " user"); cut != -1 {
		part = part[:cut]
	}
	return strings.Trim(part, " ,")
}

func getDiskSummary() string {
	if _, err := exec.LookPath("df"); err != nil {
		return ""
	}
	out, err := runQuickCmd([]string{"df", "-h", "/"}, 2*time.Second)
	if err != nil {
		return ""
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) < 2 {
		return ""
	}
	fields := strings.Fields(lines[1])
	if len(fields) < 5 {
		return ""
	}
	size := fields[1]
	used := fields[2]
	usePct := fields[4]
	return fmt.Sprintf("/ %s used %s (%s)", size, used, usePct)
}

func getNetSummary() string {
	rate, ok := getNetRateKB()
	if !ok {
		return ""
	}
	iface := getPrimaryIface()
	if iface == "" {
		iface = "iface"
	}
	return fmt.Sprintf("%s %s", iface, FormatRate(rate))
}

func getPrimaryIface() string {
	if data, err := os.ReadFile("/proc/net/dev"); err == nil {
		if iface := firstIfaceLinux(data); iface != "" {
			return iface
		}
	}
	if _, err := exec.LookPath("netstat"); err == nil {
		if iface := firstIfaceDarwin(); iface != "" {
			return iface
		}
	}
	return ""
}

func firstIfaceLinux(data []byte) string {
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if !strings.Contains(line, ":") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		iface := strings.TrimSpace(parts[0])
		if iface == loStr || strings.HasPrefix(iface, loStr) {
			continue
		}
		return iface
	}
	return ""
}

func firstIfaceDarwin() string {
	out, err := runQuickCmd([]string{"netstat", "-ib"}, 2*time.Second)
	if err != nil {
		return ""
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) < 2 {
		return ""
	}
	header := strings.Fields(lines[0])
	nIdx := indexOf(header, "Name")
	if nIdx == -1 {
		return ""
	}
	for _, line := range lines[1:] {
		fields := strings.Fields(line)
		if len(fields) <= nIdx {
			continue
		}
		iface := fields[nIdx]
		if iface == lo0Str || strings.HasPrefix(iface, loStr) {
			continue
		}
		return iface
	}
	return ""
}

func getLoadAvg() (float64, bool) {
	if _, err := exec.LookPath("uptime"); err != nil {
		return 0, false
	}
	out, err := runQuickCmd([]string{"uptime"}, 2*time.Second)
	if err != nil {
		return 0, false
	}
	line := strings.TrimSpace(out)
	idx := strings.Index(line, "load average")
	if idx == -1 {
		idx = strings.Index(line, "load averages")
	}
	if idx == -1 {
		return 0, false
	}
	part := line[idx:]
	parts := strings.FieldsFunc(part, func(r rune) bool {
		return r == ':' || r == ','
	})
	if len(parts) < 2 {
		return 0, false
	}
	loadStr := strings.TrimSpace(parts[1])
	loadStr = strings.TrimSuffix(loadStr, ",")
	load, err := parseFloat(loadStr)
	if err != nil {
		return 0, false
	}
	return load, true
}

func getCPUUsage() (float64, bool) {
	if _, err := exec.LookPath("vmstat"); err == nil {
		if cpu, ok := cpuFromVmstat(); ok {
			return cpu, true
		}
	}
	if _, err := exec.LookPath("mpstat"); err == nil {
		if cpu, ok := cpuFromMpstat(); ok {
			return cpu, true
		}
	}
	return 0, false
}

func cpuFromVmstat() (float64, bool) {
	// On macOS, vmstat 1 2 gives a good average.
	// On Linux, vmstat gives it in the last line.
	out, err := runQuickCmd([]string{"vmstat", "1", "2"}, 3*time.Second)
	if err != nil {
		// Fallback to single shot if 1 2 fails
		out, err = runQuickCmd([]string{"vmstat"}, 2*time.Second)
		if err != nil {
			return 0, false
		}
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) < 3 {
		return 0, false
	}

	// We look for the last line of output
	valuesLine := lines[len(lines)-1]
	headerLine := ""
	// Find the header line (usually the one with "id" or "us")
	for i := len(lines) - 2; i >= 0; i-- {
		if strings.Contains(lines[i], "id") || strings.Contains(lines[i], "us") {
			headerLine = lines[i]
			break
		}
	}

	if headerLine == "" {
		return 0, false
	}

	hFields := strings.Fields(headerLine)
	vFields := strings.Fields(valuesLine)

	// In some vmstat versions (macOS), the header and values might not align perfectly in Fields()
	// because of sub-headers. We try to find "id" from the end.
	idx := -1
	for i := len(hFields) - 1; i >= 0; i-- {
		if hFields[i] == "id" {
			idx = i
			break
		}
	}

	if idx == -1 || idx >= len(vFields) {
		return 0, false
	}

	idle, err := parseFloat(vFields[idx])
	if err != nil {
		return 0, false
	}
	cpu := 100 - idle
	if cpu < 0 {
		cpu = 0
	}
	if cpu > 100 {
		cpu = 100
	}
	return cpu, true
}

func cpuFromMpstat() (float64, bool) {
	out, err := runQuickCmd([]string{"mpstat", "1", "1"}, 3*time.Second)
	if err != nil {
		return 0, false
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" || strings.HasPrefix(line, "Linux") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}
		if strings.ToLower(fields[1]) != "all" {
			continue
		}
		idleStr := fields[len(fields)-1]
		idle, err := parseFloat(idleStr)
		if err != nil {
			continue
		}
		cpu := 100 - idle
		if cpu < 0 {
			cpu = 0
		}
		if cpu > 100 {
			cpu = 100
		}
		return cpu, true
	}
	return 0, false
}

func getMemUsage() (float64, bool) {
	if _, err := exec.LookPath("free"); err == nil {
		return memFromFree()
	}
	if _, err := exec.LookPath("vm_stat"); err == nil {
		return memFromVmStat()
	}
	return 0, false
}

func memFromFree() (float64, bool) {
	out, err := runQuickCmd([]string{"free", "-m"}, 2*time.Second)
	if err != nil {
		return 0, false
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "Mem:") {
			fields := strings.Fields(line)
			if len(fields) < 3 {
				return 0, false
			}
			total, err := parseFloat(fields[1])
			if err != nil || total == 0 {
				return 0, false
			}
			used, err := parseFloat(fields[2])
			if err != nil {
				return 0, false
			}
			return (used / total) * 100, true
		}
	}
	return 0, false
}

func memFromVmStat() (float64, bool) {
	out, err := runQuickCmd([]string{"vm_stat"}, 2*time.Second)
	if err != nil {
		return 0, false
	}
	lines := strings.Split(out, "\n")
	var free, active, inactive, wired, compressed float64

	for _, line := range lines {
		parts := strings.Split(line, ":")
		if len(parts) < 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		valStr := strings.TrimSuffix(strings.TrimSpace(parts[1]), ".")
		val, _ := parseFloat(valStr)

		switch key {
		case "Pages free":
			free = val
		case "Pages active":
			active = val
		case "Pages inactive":
			inactive = val
		case "Pages wired down":
			wired = val
		case "Pages occupied by compressor":
			compressed = val
		}
	}

	total := free + active + inactive + wired + compressed
	if total == 0 {
		return 0, false
	}
	used := active + wired + compressed
	return (used / total) * 100, true
}

var netPrevTotal uint64
var netPrevAt time.Time
var netMu sync.Mutex

func getNetRateKB() (float64, bool) {
	netMu.Lock()
	defer netMu.Unlock()

	total, ok := readNetBytes()
	if !ok {
		return 0, false
	}
	now := time.Now()
	if netPrevAt.IsZero() {
		netPrevAt = now
		netPrevTotal = total
		return 0, false
	}
	if total < netPrevTotal {
		netPrevAt = now
		netPrevTotal = total
		return 0, false
	}
	secs := now.Sub(netPrevAt).Seconds()
	if secs <= 0 {
		netPrevAt = now
		netPrevTotal = total
		return 0, false
	}
	delta := total - netPrevTotal
	netPrevAt = now
	netPrevTotal = total
	return float64(delta) / 1024.0 / secs, true
}

func readNetBytes() (uint64, bool) {
	if data, err := os.ReadFile("/proc/net/dev"); err == nil {
		if total, ok := sumNetBytesLinux(data); ok {
			return total, true
		}
	}
	if _, err := exec.LookPath("netstat"); err == nil {
		if total, ok := sumNetBytesDarwin(); ok {
			return total, true
		}
	}
	return 0, false
}

func sumNetBytesLinux(data []byte) (uint64, bool) {
	lines := strings.Split(string(data), "\n")
	var total uint64
	var found bool
	for _, line := range lines {
		if !strings.Contains(line, ":") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		iface := strings.TrimSpace(parts[0])
		if iface == loStr || strings.HasPrefix(iface, loStr) {
			continue
		}
		fields := strings.Fields(parts[1])
		if len(fields) < 16 {
			continue
		}
		rx, err := strconv.ParseUint(fields[0], 10, 64)
		if err != nil {
			continue
		}
		tx, err := strconv.ParseUint(fields[8], 10, 64)
		if err != nil {
			continue
		}
		total += rx + tx
		found = true
	}
	return total, found
}

func sumNetBytesDarwin() (uint64, bool) {
	out, err := runQuickCmd([]string{"netstat", "-ib"}, 2*time.Second)
	if err != nil {
		return 0, false
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) < 2 {
		return 0, false
	}
	header := strings.Fields(lines[0])
	iIdx := indexOf(header, "Ibytes")
	oIdx := indexOf(header, "Obytes")
	nIdx := indexOf(header, "Name")
	if iIdx == -1 || oIdx == -1 || nIdx == -1 {
		return 0, false
	}
	var total uint64
	var found bool
	for _, line := range lines[1:] {
		fields := strings.Fields(line)
		if len(fields) <= oIdx || len(fields) <= iIdx || len(fields) <= nIdx {
			continue
		}
		iface := fields[nIdx]
		if iface == lo0Str || strings.HasPrefix(iface, loStr) {
			continue
		}
		ib, err := strconv.ParseUint(fields[iIdx], 10, 64)
		if err != nil {
			continue
		}
		ob, err := strconv.ParseUint(fields[oIdx], 10, 64)
		if err != nil {
			continue
		}
		total += ib + ob
		found = true
	}
	return total, found
}
