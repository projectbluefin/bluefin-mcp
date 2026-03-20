package system

import (
	"context"
	"errors"
	"strings"

	"github.com/projectbluefin/bluefin-mcp/internal/cli"
)

// HardwareReport is the output of GetHardwareReport.
//
// Division of responsibility with linux-mcp-server:
//   - CPU model, core count, frequency, load          → linux-mcp-server: get_cpu_information
//   - RAM and swap usage                               → linux-mcp-server: get_memory_information
//   - Disk / filesystem usage                          → linux-mcp-server: get_disk_usage
//   - General hardware inventory (lspci, lsusb, lscpu) → linux-mcp-server: get_hardware_information
//
// This tool adds what linux-mcp-server cannot provide:
//   - lspci -nnk: numeric PCI vendor:device IDs + loaded kernel modules
//     (linux-mcp-server runs bare lspci — no numeric IDs, no driver info)
//   - LiveCD detection via /proc/cmdline (rd.live.image flag)
//   - Laptop model/chassis/firmware via /sys/class/dmi/id/ (no root required)
//   - Bluefin variant compatibility assessment (Nvidia GPU vs. image variant)
//   - Static known-incompatible hardware flagging (Broadcom WiFi vendor 14e4)
type HardwareReport struct {
	System          SystemMeta    `json:"system"`
	CriticalDevices []PCIDevice   `json:"critical_devices"`
	VariantCompat   VariantCompat `json:"variant_compat"`
	Issues          []string      `json:"issues"`
}

// SystemMeta holds laptop/desktop identification data read from sysfs.
// Does not require root. Sourced from /sys/class/dmi/id/.
type SystemMeta struct {
	Model           string `json:"model"`
	Chassis         string `json:"chassis"`          // human-readable: "notebook", "desktop", etc.
	FirmwareVersion string `json:"firmware_version"` // BIOS/UEFI version string
	IsLiveImage     bool   `json:"is_live_image"`    // true when booted from Live ISO
}

// PCIDevice represents a GPU or network adapter from lspci -nnk output.
// Only devices relevant to Bluefin compatibility are included.
type PCIDevice struct {
	Type        string `json:"type"`          // "gpu" or "network"
	Name        string `json:"name"`          // human-readable device name
	PCIID       string `json:"pci_id"`        // "vendor:device" e.g. "14e4:43a0"
	Driver      string `json:"driver"`        // loaded kernel module, or "" if none
	DriverActive bool  `json:"driver_active"` // true if kernel module is loaded
}

// VariantCompat reports whether the running Bluefin variant matches detected hardware.
type VariantCompat struct {
	CurrentVariant string `json:"current_variant"`
	NvidiaDetected bool   `json:"nvidia_detected"`
	VariantMatch   bool   `json:"variant_match"`
	Recommendation string `json:"recommendation,omitempty"`
}

// PCI vendor IDs used in the static compatibility assessment.
// This list is embedded at compile time; it is not a dynamic database.
const (
	vendorNvidia   = "10de" // Nvidia Corporation
	vendorBroadcom = "14e4" // Broadcom Inc.
	vendorAMD      = "1002" // Advanced Micro Devices
	vendorIntel    = "8086" // Intel Corporation
)

// PCI class codes for device type classification.
const (
	pciClassVGA     = "0300" // VGA compatible controller
	pciClass3D      = "0302" // 3D controller
	pciClassNetwork = "0280" // Network controller (includes WiFi)
	pciClassEthernet = "0200" // Ethernet controller
)

// chassisNames maps DMI chassis type numbers to human-readable strings.
// Source: SMBIOS spec, table 17.
var chassisNames = map[string]string{
	"1": "other", "2": "unknown", "3": "desktop", "4": "low-profile-desktop",
	"8": "portable", "9": "laptop", "10": "notebook", "11": "hand-held",
	"14": "sub-notebook", "30": "tablet", "31": "convertible", "32": "detachable",
}

// GetHardwareReport returns a hardware compatibility assessment for this Bluefin system.
//
// For general hardware facts (CPU, memory, disk, full PCI/USB inventory) use
// linux-mcp-server's get_hardware_information, get_cpu_information,
// get_memory_information, and get_disk_usage instead.
func GetHardwareReport(ctx context.Context, runner cli.CommandRunner) (*HardwareReport, error) {
	report := &HardwareReport{}

	// System metadata from world-readable sysfs (no root required).
	report.System = readSystemMeta(ctx, runner)

	// LiveCD detection: rd.live.image in kernel command line.
	if cmdline, err := runner.Run(ctx, "cat", []string{"/proc/cmdline"}); err == nil {
		report.System.IsLiveImage = strings.Contains(string(cmdline), "rd.live.image")
	}

	// PCI devices with numeric vendor:device IDs and loaded kernel modules.
	// linux-mcp-server runs bare lspci — no numeric IDs, no driver info.
	// -nnk adds numeric vendor:device IDs and loaded kernel modules — more
	// useful for diagnosis than linux-mcp-server's bare lspci output.
	lspciOut, err := runner.Run(ctx, "lspci", []string{"-nnk"})
	if err != nil && !errors.Is(err, cli.ErrNotInstalled) {
		return nil, err
	}
	if err == nil {
		report.CriticalDevices = parseLspciNNK(string(lspciOut))
	}

	// Variant compatibility: cross-reference detected GPU vendor with the
	// running Bluefin variant. Calls the existing GetSystemStatus function.
	status, err := GetSystemStatus(ctx, runner)
	if err == nil {
		report.VariantCompat = assessVariantCompat(status.Variant, report.CriticalDevices)
	} else {
		report.VariantCompat = VariantCompat{CurrentVariant: "unknown", VariantMatch: true}
	}

	report.Issues = buildIssues(report.CriticalDevices, report.VariantCompat)
	return report, nil
}

func readSystemMeta(ctx context.Context, runner cli.CommandRunner) SystemMeta {
	meta := SystemMeta{}
	if b, err := runner.Run(ctx, "cat", []string{"/sys/class/dmi/id/product_name"}); err == nil {
		meta.Model = strings.TrimSpace(string(b))
	}
	if b, err := runner.Run(ctx, "cat", []string{"/sys/class/dmi/id/chassis_type"}); err == nil {
		num := strings.TrimSpace(string(b))
		if name, ok := chassisNames[num]; ok {
			meta.Chassis = name
		} else {
			meta.Chassis = num
		}
	}
	if b, err := runner.Run(ctx, "cat", []string{"/sys/class/dmi/id/bios_version"}); err == nil {
		meta.FirmwareVersion = strings.TrimSpace(string(b))
	}
	return meta
}

// parseLspciNNK parses `lspci -nnk` output.
// Only GPU (class 0300/0302) and network (class 0280/0200) devices are included.
// All other PCI devices are omitted — linux-mcp-server covers the full inventory.
func parseLspciNNK(output string) []PCIDevice {
	var devices []PCIDevice
	var current *PCIDevice

	for _, line := range strings.Split(output, "\n") {
		if line == "" {
			if current != nil && current.Type != "" {
				devices = append(devices, *current)
				current = nil
			}
			continue
		}

		if !strings.HasPrefix(line, "\t") {
			// Flush previous device
			if current != nil && current.Type != "" {
				devices = append(devices, *current)
			}
			current = parseLspciDeviceLine(line)
			continue
		}

		// Tab-indented continuation lines (driver info, subsystem, etc.)
		if current == nil || current.Type == "" {
			continue
		}
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "Kernel driver in use:") {
			driver := strings.TrimSpace(strings.TrimPrefix(trimmed, "Kernel driver in use:"))
			current.Driver = driver
			current.DriverActive = driver != ""
		}
	}
	// Flush last device
	if current != nil && current.Type != "" {
		devices = append(devices, *current)
	}
	return devices
}

// parseLspciDeviceLine parses a non-indented lspci -nnk line.
// Format: "01:00.0 Class description [classid]: Vendor Name Model [vendorid:deviceid] (rev xx)"
func parseLspciDeviceLine(line string) *PCIDevice {
	dev := &PCIDevice{}

	// Extract the last [vendor:device] pair — always the PCI ID in lspci -nn output.
	pciID := extractLastBracketContent(line)
	if pciID == "" || !strings.Contains(pciID, ":") {
		return dev
	}
	dev.PCIID = pciID
	vendor := strings.SplitN(pciID, ":", 2)[0]

	// Extract the first [classid] — the PCI class code.
	classID := extractFirstBracketContent(line)

	switch {
	case classID == pciClassVGA || classID == pciClass3D:
		dev.Type = "gpu"
	case vendor == vendorNvidia || vendor == vendorAMD:
		// Some Nvidia/AMD devices use non-standard class codes.
		dev.Type = "gpu"
	case classID == pciClassNetwork || classID == pciClassEthernet:
		dev.Type = "network"
	case vendor == vendorBroadcom:
		// Broadcom WiFi adapters may use varying class codes.
		dev.Type = "network"
	default:
		return dev // not relevant to compatibility assessment
	}

	// Extract device name: text between the first "]: " and the final " [vendor:device]".
	if idx := strings.Index(line, "]: "); idx >= 0 {
		name := line[idx+3:]
		if bidx := strings.LastIndex(name, " ["); bidx >= 0 {
			name = name[:bidx]
		}
		dev.Name = strings.TrimSpace(name)
	}

	return dev
}

func extractFirstBracketContent(line string) string {
	start := strings.Index(line, "[")
	if start < 0 {
		return ""
	}
	end := strings.Index(line[start:], "]")
	if end < 0 {
		return ""
	}
	return line[start+1 : start+end]
}

func extractLastBracketContent(line string) string {
	end := strings.LastIndex(line, "]")
	if end < 0 {
		return ""
	}
	// Strip trailing " (rev xx)" that may appear after the last bracket in some outputs
	// by finding the true last "]" that contains a ":"
	content := line[:end]
	start := strings.LastIndex(content, "[")
	if start < 0 {
		return ""
	}
	return content[start+1:]
}

func assessVariantCompat(variant string, devices []PCIDevice) VariantCompat {
	vc := VariantCompat{CurrentVariant: variant}
	for _, d := range devices {
		if d.Type == "gpu" && strings.HasPrefix(d.PCIID, vendorNvidia+":") {
			vc.NvidiaDetected = true
		}
	}
	if vc.NvidiaDetected {
		switch variant {
		case "nvidia", "aurora-nvidia":
			vc.VariantMatch = true
		default:
			vc.VariantMatch = false
			vc.Recommendation = "Nvidia GPU detected on '" + variant + "' variant. " +
				"The nouveau driver is active. For full Nvidia driver support, " +
				"switch to the nvidia variant: ujust rebase-helper"
		}
	} else {
		vc.VariantMatch = true
	}
	return vc
}

func buildIssues(devices []PCIDevice, vc VariantCompat) []string {
	var issues []string
	for _, d := range devices {
		if d.Type == "network" && strings.HasPrefix(d.PCIID, vendorBroadcom+":") {
			issues = append(issues, "Broadcom WiFi adapter ["+d.PCIID+"] detected. "+
				"The proprietary wl driver is not included in the Fedora stock kernel. "+
				"WiFi will not function on this hardware with the standard Bluefin image.")
		}
	}
	if vc.NvidiaDetected && !vc.VariantMatch {
		issues = append(issues, "Nvidia GPU detected on '"+vc.CurrentVariant+"' variant. "+
			"The nouveau open-source driver is active. "+
			"For full Nvidia driver support, switch to the nvidia variant.")
	}
	return issues
}
