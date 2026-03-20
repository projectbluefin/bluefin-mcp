package system_test

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/projectbluefin/bluefin-mcp/internal/cli"
	"github.com/projectbluefin/bluefin-mcp/internal/system"
)

// hardwareMock builds a MockExecutor with all commands GetHardwareReport needs.
// bootcData is the raw JSON for bootc status --json.
// Use testdata files for lspci output; inlined bytes for sysfs/cmdline values.
func hardwareMock(lspciOut []byte, cmdline, model, chassisType, firmware string, bootcData []byte) *cli.MockExecutor {
	m := cli.NewMockExecutor()
	m.SetResponse("lspci", []string{"-nnk"}, lspciOut, nil)
	m.SetResponse("cat", []string{"/proc/cmdline"}, []byte(cmdline), nil)
	m.SetResponse("cat", []string{"/sys/class/dmi/id/product_name"}, []byte(model), nil)
	m.SetResponse("cat", []string{"/sys/class/dmi/id/chassis_type"}, []byte(chassisType), nil)
	m.SetResponse("cat", []string{"/sys/class/dmi/id/bios_version"}, []byte(firmware), nil)
	m.SetResponse("bootc", []string{"status", "--json"}, bootcData, nil)
	return m
}

func TestGetHardwareReport_NvidiaOnBaseVariant(t *testing.T) {
	lspci, err := os.ReadFile("../../testdata/lspci-nnk-nvidia-broadcom.txt")
	if err != nil {
		t.Fatalf("missing testdata: %v", err)
	}
	bootcBase, err := os.ReadFile("../../testdata/bootc-status.json")
	if err != nil {
		t.Fatalf("missing testdata: %v", err)
	}
	cmdline, err := os.ReadFile("../../testdata/cmdline-installed.txt")
	if err != nil {
		t.Fatalf("missing testdata: %v", err)
	}

	mock := hardwareMock(lspci, strings.TrimSpace(string(cmdline)),
		"Dell XPS 15 9510", "10", "3.4.0", bootcBase)

	report, err := system.GetHardwareReport(context.Background(), mock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !report.VariantCompat.NvidiaDetected {
		t.Error("expected NvidiaDetected=true for Nvidia GPU in lspci output")
	}
	if report.VariantCompat.VariantMatch {
		t.Error("expected VariantMatch=false: Nvidia GPU on base variant")
	}
	if report.VariantCompat.Recommendation == "" {
		t.Error("expected non-empty Recommendation for variant mismatch")
	}

	var hasNvidiaIssue bool
	for _, iss := range report.Issues {
		if strings.Contains(strings.ToLower(iss), "nvidia") {
			hasNvidiaIssue = true
		}
	}
	if !hasNvidiaIssue {
		t.Errorf("expected issue mentioning nvidia, got: %v", report.Issues)
	}
}

func TestGetHardwareReport_NvidiaOnNvidiaVariant(t *testing.T) {
	lspci, _ := os.ReadFile("../../testdata/lspci-nnk-nvidia-broadcom.txt")
	bootcNvidia, _ := os.ReadFile("../../testdata/bootc-status-nvidia.json")
	cmdline, _ := os.ReadFile("../../testdata/cmdline-installed.txt")

	mock := hardwareMock(lspci, strings.TrimSpace(string(cmdline)),
		"Dell XPS 15 9510", "10", "3.4.0", bootcNvidia)

	report, err := system.GetHardwareReport(context.Background(), mock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !report.VariantCompat.NvidiaDetected {
		t.Error("expected NvidiaDetected=true")
	}
	if !report.VariantCompat.VariantMatch {
		t.Error("expected VariantMatch=true: Nvidia GPU on nvidia variant")
	}
}

func TestGetHardwareReport_BroadcomFlagged(t *testing.T) {
	lspci, _ := os.ReadFile("../../testdata/lspci-nnk-nvidia-broadcom.txt")
	bootcBase, _ := os.ReadFile("../../testdata/bootc-status.json")
	cmdline, _ := os.ReadFile("../../testdata/cmdline-installed.txt")

	mock := hardwareMock(lspci, strings.TrimSpace(string(cmdline)),
		"Dell XPS 15 9510", "10", "3.4.0", bootcBase)

	report, err := system.GetHardwareReport(context.Background(), mock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var broadcomDev *system.PCIDevice
	for i := range report.CriticalDevices {
		if report.CriticalDevices[i].Type == "network" &&
			strings.HasPrefix(report.CriticalDevices[i].PCIID, "14e4:") {
			broadcomDev = &report.CriticalDevices[i]
		}
	}
	if broadcomDev == nil {
		t.Fatal("expected Broadcom network device in CriticalDevices")
	}
	if broadcomDev.DriverActive {
		t.Error("Broadcom WiFi: expected DriverActive=false (no wl in stock Fedora kernel)")
	}

	var hasBroadcomIssue bool
	for _, iss := range report.Issues {
		if strings.Contains(strings.ToLower(iss), "broadcom") {
			hasBroadcomIssue = true
		}
	}
	if !hasBroadcomIssue {
		t.Errorf("expected issue mentioning broadcom, got: %v", report.Issues)
	}
}

func TestGetHardwareReport_LiveCDDetected(t *testing.T) {
	lspci, _ := os.ReadFile("../../testdata/lspci-nnk-intel-igpu.txt")
	cmdline, _ := os.ReadFile("../../testdata/cmdline-livecd.txt")

	mock := hardwareMock(lspci, strings.TrimSpace(string(cmdline)),
		"Framework Laptop 13", "10", "03.04",
		// No bootc on LiveCD — simulate ErrNotInstalled
		nil)
	mock.SetResponse("bootc", []string{"status", "--json"}, nil, cli.ErrNotInstalled)
	mock.SetResponse("rpm-ostree", []string{"status", "--json"}, nil, cli.ErrNotInstalled)

	report, err := system.GetHardwareReport(context.Background(), mock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !report.System.IsLiveImage {
		t.Error("expected IsLiveImage=true when cmdline contains rd.live.image")
	}
}

func TestGetHardwareReport_InstalledNotLiveCD(t *testing.T) {
	lspci, _ := os.ReadFile("../../testdata/lspci-nnk-amd-intel.txt")
	bootcBase, _ := os.ReadFile("../../testdata/bootc-status.json")
	cmdline, _ := os.ReadFile("../../testdata/cmdline-installed.txt")

	mock := hardwareMock(lspci, strings.TrimSpace(string(cmdline)),
		"ASUS ROG Zephyrus G14", "10", "312", bootcBase)

	report, err := system.GetHardwareReport(context.Background(), mock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.System.IsLiveImage {
		t.Error("expected IsLiveImage=false for installed system cmdline")
	}
}

func TestGetHardwareReport_FullySupportedHardware(t *testing.T) {
	lspci, _ := os.ReadFile("../../testdata/lspci-nnk-amd-intel.txt")
	bootcBase, _ := os.ReadFile("../../testdata/bootc-status.json")
	cmdline, _ := os.ReadFile("../../testdata/cmdline-installed.txt")

	mock := hardwareMock(lspci, strings.TrimSpace(string(cmdline)),
		"ASUS ROG Zephyrus G14", "10", "312", bootcBase)

	report, err := system.GetHardwareReport(context.Background(), mock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(report.Issues) != 0 {
		t.Errorf("expected 0 issues for AMD GPU + Intel WiFi, got: %v", report.Issues)
	}
}

func TestGetHardwareReport_LspciNotInstalled(t *testing.T) {
	bootcBase, _ := os.ReadFile("../../testdata/bootc-status.json")

	m := cli.NewMockExecutor()
	m.SetResponse("lspci", []string{"-nnk"}, nil, cli.ErrNotInstalled)
	m.SetResponse("cat", []string{"/proc/cmdline"}, []byte("BOOT_IMAGE=... ostree=/ostree/boot.0/..."), nil)
	m.SetResponse("cat", []string{"/sys/class/dmi/id/product_name"}, []byte("Unknown"), nil)
	m.SetResponse("cat", []string{"/sys/class/dmi/id/chassis_type"}, []byte("1"), nil)
	m.SetResponse("cat", []string{"/sys/class/dmi/id/bios_version"}, []byte("unknown"), nil)
	m.SetResponse("bootc", []string{"status", "--json"}, bootcBase, nil)

	report, err := system.GetHardwareReport(context.Background(), m)
	if err != nil {
		t.Fatalf("expected graceful degrade when lspci absent, got error: %v", err)
	}
	if len(report.CriticalDevices) != 0 {
		t.Errorf("expected empty CriticalDevices when lspci absent, got %d", len(report.CriticalDevices))
	}
}

func TestGetHardwareReport_SystemMetaParsed(t *testing.T) {
	lspci, _ := os.ReadFile("../../testdata/lspci-nnk-intel-igpu.txt")
	bootcBase, _ := os.ReadFile("../../testdata/bootc-status.json")
	cmdline, _ := os.ReadFile("../../testdata/cmdline-installed.txt")

	mock := hardwareMock(lspci, strings.TrimSpace(string(cmdline)),
		"Framework Laptop 13", "10", "03.04", bootcBase)

	report, err := system.GetHardwareReport(context.Background(), mock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.System.Model != "Framework Laptop 13" {
		t.Errorf("expected model 'Framework Laptop 13', got %q", report.System.Model)
	}
	if report.System.Chassis != "notebook" {
		t.Errorf("expected chassis 'notebook' for type 10, got %q", report.System.Chassis)
	}
	if report.System.FirmwareVersion != "03.04" {
		t.Errorf("expected firmware '03.04', got %q", report.System.FirmwareVersion)
	}
}
