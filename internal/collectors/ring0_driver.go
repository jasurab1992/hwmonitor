//go:build windows

package collectors

import (
	_ "embed"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

//go:embed drivers/WinRing0x64.sys
var ring0SysData []byte

const (
	ring0ServiceName = "WinRing0_1_2_0"
	ring0DevicePath  = `\\.\WinRing0_1_2_0`
)

var (
	ring0Once    sync.Once
	ring0OK      bool
	ring0TempSys string
	ring0Cleanup func()
)

// ensureRing0 installs and starts the embedded WinRing0 driver exactly once.
// Returns true if the device \\.\WinRing0_1_2_0 is ready to use.
func ensureRing0() bool {
	ring0Once.Do(func() {
		ring0OK, ring0Cleanup = startRing0Driver()
	})
	return ring0OK
}

// CleanupRing0 stops the service and removes the temp .sys file.
// Call on application exit.
func CleanupRing0() {
	if ring0Cleanup != nil {
		ring0Cleanup()
	}
}

func startRing0Driver() (ok bool, cleanup func()) {
	// Step 1: check if device already exists (LHM / OHM / CPU-Z installed it)
	if deviceExists() {
		log.Printf("ring0: device already present (external driver)")
		return true, func() {}
	}

	// Step 2: extract embedded .sys to temp
	tmpSys := filepath.Join(os.TempDir(), "hwmon_ring0.sys")
	if err := os.WriteFile(tmpSys, ring0SysData, 0600); err != nil {
		log.Printf("ring0: failed to extract driver: %v", err)
		return false, nil
	}

	// Step 3: connect to SCM
	m, err := mgr.Connect()
	if err != nil {
		log.Printf("ring0: SCM connect failed (need Administrator): %v", err)
		os.Remove(tmpSys)
		return false, nil
	}
	defer m.Disconnect()

	// Step 4: create or open the service
	s, err := m.OpenService(ring0ServiceName)
	if err != nil {
		s, err = m.CreateService(ring0ServiceName, tmpSys, mgr.Config{
			ServiceType:  windows.SERVICE_KERNEL_DRIVER,
			StartType:    mgr.StartManual,
			ErrorControl: mgr.ErrorNormal,
			DisplayName:  "WinRing0 Hardware Access",
		})
		if err != nil {
			log.Printf("ring0: CreateService failed: %v", err)
			os.Remove(tmpSys)
			return false, nil
		}
		log.Printf("ring0: service created")
	}
	s.Close()

	// Step 5: start the service
	if err := startService(m); err != nil {
		log.Printf("ring0: start failed: %v", err)
		removeService(m)
		os.Remove(tmpSys)
		return false, nil
	}

	log.Printf("ring0: driver started")
	ring0TempSys = tmpSys

	cleanup = func() {
		m2, err := mgr.Connect()
		if err != nil {
			return
		}
		defer m2.Disconnect()
		stopService(m2)
		removeService(m2)
		if ring0TempSys != "" {
			os.Remove(ring0TempSys)
		}
	}
	return true, cleanup
}

func deviceExists() bool {
	h, err := openRing0Device()
	if err != nil {
		return false
	}
	windows.CloseHandle(h)
	return true
}

func startService(m *mgr.Mgr) error {
	s, err := m.OpenService(ring0ServiceName)
	if err != nil {
		return err
	}
	defer s.Close()

	// If a previous run left the service disabled, re-enable it.
	if cfg, err := s.Config(); err == nil && cfg.StartType == mgr.StartDisabled {
		cfg.StartType = mgr.StartManual
		_ = s.UpdateConfig(cfg)
	}

	if err := s.Start(); err != nil {
		// Already running is fine
		st, _ := s.Query()
		if st.State == svc.Running {
			return nil
		}
		return err
	}
	// Wait up to 2s for running state
	for i := 0; i < 20; i++ {
		time.Sleep(100 * time.Millisecond)
		st, _ := s.Query()
		if st.State == svc.Running {
			return nil
		}
	}
	return nil
}

func stopService(m *mgr.Mgr) {
	s, err := m.OpenService(ring0ServiceName)
	if err != nil {
		return
	}
	defer s.Close()
	s.Control(svc.Stop)
	time.Sleep(500 * time.Millisecond)
}

func removeService(m *mgr.Mgr) {
	s, err := m.OpenService(ring0ServiceName)
	if err != nil {
		return
	}
	defer s.Close()
	s.Delete()
}
