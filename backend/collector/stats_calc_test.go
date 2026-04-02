package collector

import (
	"testing"

	containertypes "github.com/docker/docker/api/types/container"
)

// --- calcCPUPercent ---

func TestCalcCPUPercent(t *testing.T) {
	tests := []struct {
		name     string
		stats    *containertypes.StatsResponse
		wantZero bool
		want     float64
	}{
		{
			name: "normal calculation with OnlineCPUs",
			stats: &containertypes.StatsResponse{
				CPUStats: containertypes.CPUStats{
					CPUUsage:    containertypes.CPUUsage{TotalUsage: 5_000_000_000},
					SystemUsage: 20_000_000_000,
					OnlineCPUs:  4,
				},
				PreCPUStats: containertypes.CPUStats{
					CPUUsage:    containertypes.CPUUsage{TotalUsage: 4_000_000_000},
					SystemUsage: 10_000_000_000,
				},
			},
			// cpuDelta=1e9, systemDelta=10e9 => (1/10)*4*100 = 40
			want: 40.0,
		},
		{
			name: "zero system delta returns 0",
			stats: &containertypes.StatsResponse{
				CPUStats: containertypes.CPUStats{
					CPUUsage:    containertypes.CPUUsage{TotalUsage: 5_000_000_000},
					SystemUsage: 10_000_000_000,
					OnlineCPUs:  2,
				},
				PreCPUStats: containertypes.CPUStats{
					CPUUsage:    containertypes.CPUUsage{TotalUsage: 4_000_000_000},
					SystemUsage: 10_000_000_000,
				},
			},
			wantZero: true,
		},
		{
			name: "zero cpu delta returns 0",
			stats: &containertypes.StatsResponse{
				CPUStats: containertypes.CPUStats{
					CPUUsage:    containertypes.CPUUsage{TotalUsage: 5_000_000_000},
					SystemUsage: 20_000_000_000,
					OnlineCPUs:  2,
				},
				PreCPUStats: containertypes.CPUStats{
					CPUUsage:    containertypes.CPUUsage{TotalUsage: 5_000_000_000},
					SystemUsage: 10_000_000_000,
				},
			},
			// cpuDelta=0, systemDelta=10e9 => (0/10e9)*2*100 = 0
			want: 0.0,
		},
		{
			name: "both deltas zero returns 0",
			stats: &containertypes.StatsResponse{
				CPUStats: containertypes.CPUStats{
					CPUUsage:    containertypes.CPUUsage{TotalUsage: 5_000_000_000},
					SystemUsage: 10_000_000_000,
					OnlineCPUs:  4,
				},
				PreCPUStats: containertypes.CPUStats{
					CPUUsage:    containertypes.CPUUsage{TotalUsage: 5_000_000_000},
					SystemUsage: 10_000_000_000,
				},
			},
			wantZero: true,
		},
		{
			name: "fallback to PercpuUsage length when OnlineCPUs is 0",
			stats: &containertypes.StatsResponse{
				CPUStats: containertypes.CPUStats{
					CPUUsage: containertypes.CPUUsage{
						TotalUsage:  5_000_000_000,
						PercpuUsage: []uint64{1, 2, 3, 4, 5, 6, 7, 8},
					},
					SystemUsage: 20_000_000_000,
					OnlineCPUs:  0,
				},
				PreCPUStats: containertypes.CPUStats{
					CPUUsage:    containertypes.CPUUsage{TotalUsage: 4_000_000_000},
					SystemUsage: 10_000_000_000,
				},
			},
			// cpuDelta=1e9, systemDelta=10e9, cpus=8 => (1/10)*8*100 = 80
			want: 80.0,
		},
		{
			name: "fallback to 1 when both OnlineCPUs and PercpuUsage are empty",
			stats: &containertypes.StatsResponse{
				CPUStats: containertypes.CPUStats{
					CPUUsage:    containertypes.CPUUsage{TotalUsage: 5_000_000_000},
					SystemUsage: 20_000_000_000,
					OnlineCPUs:  0,
				},
				PreCPUStats: containertypes.CPUStats{
					CPUUsage:    containertypes.CPUUsage{TotalUsage: 4_000_000_000},
					SystemUsage: 10_000_000_000,
				},
			},
			// cpuDelta=1e9, systemDelta=10e9, cpus=1 => (1/10)*1*100 = 10
			want: 10.0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := calcCPUPercent(tc.stats)
			if tc.wantZero {
				if got != 0 {
					t.Errorf("expected 0, got %f", got)
				}
				return
			}
			if got != tc.want {
				t.Errorf("expected %f, got %f", tc.want, got)
			}
		})
	}
}

// --- calcCPUThrottle ---

func TestCalcCPUThrottle(t *testing.T) {
	tests := []struct {
		name  string
		stats *containertypes.StatsResponse
		want  float64
	}{
		{
			name: "normal throttle calculation",
			stats: &containertypes.StatsResponse{
				CPUStats: containertypes.CPUStats{
					ThrottlingData: containertypes.ThrottlingData{
						Periods:          100,
						ThrottledPeriods: 25,
					},
				},
			},
			want: 25.0,
		},
		{
			name: "zero periods returns 0",
			stats: &containertypes.StatsResponse{
				CPUStats: containertypes.CPUStats{
					ThrottlingData: containertypes.ThrottlingData{
						Periods:          0,
						ThrottledPeriods: 10,
					},
				},
			},
			want: 0.0,
		},
		{
			name: "100 percent throttle",
			stats: &containertypes.StatsResponse{
				CPUStats: containertypes.CPUStats{
					ThrottlingData: containertypes.ThrottlingData{
						Periods:          50,
						ThrottledPeriods: 50,
					},
				},
			},
			want: 100.0,
		},
		{
			name: "partial throttle",
			stats: &containertypes.StatsResponse{
				CPUStats: containertypes.CPUStats{
					ThrottlingData: containertypes.ThrottlingData{
						Periods:          200,
						ThrottledPeriods: 1,
					},
				},
			},
			want: 0.5,
		},
		{
			name:  "no throttling configured",
			stats: &containertypes.StatsResponse{},
			want:  0.0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := calcCPUThrottle(tc.stats)
			if got != tc.want {
				t.Errorf("expected %f, got %f", tc.want, got)
			}
		})
	}
}

// --- calcMemUsage ---

func TestCalcMemUsage(t *testing.T) {
	tests := []struct {
		name string
		mem  containertypes.MemoryStats
		want uint64
	}{
		{
			name: "zero usage returns 0",
			mem:  containertypes.MemoryStats{Usage: 0},
			want: 0,
		},
		{
			name: "cgroup v1 subtracts inactive_file",
			mem: containertypes.MemoryStats{
				Usage: 100_000_000,
				Stats: map[string]uint64{
					"inactive_file": 30_000_000,
				},
			},
			want: 70_000_000,
		},
		{
			name: "cgroup v2 no Stats map uses Usage directly",
			mem: containertypes.MemoryStats{
				Usage: 50_000_000,
			},
			want: 50_000_000,
		},
		{
			name: "cgroup v2 empty Stats map uses Usage directly",
			mem: containertypes.MemoryStats{
				Usage: 50_000_000,
				Stats: map[string]uint64{},
			},
			want: 50_000_000,
		},
		{
			name: "inactive_file greater than usage returns Usage directly",
			mem: containertypes.MemoryStats{
				Usage: 10_000_000,
				Stats: map[string]uint64{
					"inactive_file": 20_000_000,
				},
			},
			// inactive_file >= usage, so the condition `inactiveFile < mem.Usage` is false
			want: 10_000_000,
		},
		{
			name: "inactive_file equal to usage returns Usage directly",
			mem: containertypes.MemoryStats{
				Usage: 10_000_000,
				Stats: map[string]uint64{
					"inactive_file": 10_000_000,
				},
			},
			// inactive_file == usage, condition is not met (not strictly less than)
			want: 10_000_000,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := calcMemUsage(tc.mem)
			if got != tc.want {
				t.Errorf("expected %d, got %d", tc.want, got)
			}
		})
	}
}

// --- sumNetworkIO ---

func TestSumNetworkIO(t *testing.T) {
	tests := []struct {
		name                 string
		networks             map[string]containertypes.NetworkStats
		wantRx, wantTx       uint64
		wantRxErr, wantTxErr uint64
	}{
		{
			name:     "nil map returns zeros",
			networks: nil,
		},
		{
			name:     "empty map returns zeros",
			networks: map[string]containertypes.NetworkStats{},
		},
		{
			name: "single interface",
			networks: map[string]containertypes.NetworkStats{
				"eth0": {RxBytes: 1000, TxBytes: 2000, RxErrors: 1, TxErrors: 2},
			},
			wantRx: 1000, wantTx: 2000, wantRxErr: 1, wantTxErr: 2,
		},
		{
			name: "multiple interfaces accumulate",
			networks: map[string]containertypes.NetworkStats{
				"eth0": {RxBytes: 1000, TxBytes: 2000, RxErrors: 1, TxErrors: 0},
				"eth1": {RxBytes: 3000, TxBytes: 4000, RxErrors: 0, TxErrors: 5},
				"lo":   {RxBytes: 500, TxBytes: 500, RxErrors: 0, TxErrors: 0},
			},
			wantRx: 4500, wantTx: 6500, wantRxErr: 1, wantTxErr: 5,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rx, tx, rxErr, txErr := sumNetworkIO(tc.networks)
			if rx != tc.wantRx {
				t.Errorf("rx: expected %d, got %d", tc.wantRx, rx)
			}
			if tx != tc.wantTx {
				t.Errorf("tx: expected %d, got %d", tc.wantTx, tx)
			}
			if rxErr != tc.wantRxErr {
				t.Errorf("rxErr: expected %d, got %d", tc.wantRxErr, rxErr)
			}
			if txErr != tc.wantTxErr {
				t.Errorf("txErr: expected %d, got %d", tc.wantTxErr, txErr)
			}
		})
	}
}

// --- sumBlockIO ---

func TestSumBlockIO(t *testing.T) {
	tests := []struct {
		name      string
		blkio     containertypes.BlkioStats
		wantRead  uint64
		wantWrite uint64
	}{
		{
			name:  "empty entries",
			blkio: containertypes.BlkioStats{},
		},
		{
			name: "read and write lowercase",
			blkio: containertypes.BlkioStats{
				IoServiceBytesRecursive: []containertypes.BlkioStatEntry{
					{Op: "read", Value: 1024},
					{Op: "write", Value: 2048},
				},
			},
			wantRead: 1024, wantWrite: 2048,
		},
		{
			name: "read and write capitalized",
			blkio: containertypes.BlkioStats{
				IoServiceBytesRecursive: []containertypes.BlkioStatEntry{
					{Op: "Read", Value: 4096},
					{Op: "Write", Value: 8192},
				},
			},
			wantRead: 4096, wantWrite: 8192,
		},
		{
			name: "unknown ops are ignored",
			blkio: containertypes.BlkioStats{
				IoServiceBytesRecursive: []containertypes.BlkioStatEntry{
					{Op: "read", Value: 100},
					{Op: "sync", Value: 999},
					{Op: "async", Value: 888},
					{Op: "total", Value: 777},
					{Op: "write", Value: 200},
				},
			},
			wantRead: 100, wantWrite: 200,
		},
		{
			name: "multiple devices accumulate",
			blkio: containertypes.BlkioStats{
				IoServiceBytesRecursive: []containertypes.BlkioStatEntry{
					{Op: "Read", Value: 1000},
					{Op: "Write", Value: 2000},
					{Op: "read", Value: 3000},
					{Op: "write", Value: 4000},
				},
			},
			wantRead: 4000, wantWrite: 6000,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			read, write := sumBlockIO(tc.blkio)
			if read != tc.wantRead {
				t.Errorf("read: expected %d, got %d", tc.wantRead, read)
			}
			if write != tc.wantWrite {
				t.Errorf("write: expected %d, got %d", tc.wantWrite, write)
			}
		})
	}
}
