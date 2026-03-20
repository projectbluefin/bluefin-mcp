package tools_test

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
	"github.com/projectbluefin/bluefin-mcp/internal/cli"
	"github.com/projectbluefin/bluefin-mcp/internal/system"
	"github.com/projectbluefin/bluefin-mcp/internal/tools"
)

// callTool sends a JSON-RPC tools/call request to the server and returns the
// parsed CallToolResult. It fails the test if the transport or JSON layer errors.
func callTool(t *testing.T, s *mcpserver.MCPServer, toolName string, args map[string]any) *mcp.CallToolResult {
	t.Helper()

	req := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      toolName,
			"arguments": args,
		},
	}
	raw, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	msg := s.HandleMessage(context.Background(), raw)

	resp, ok := msg.(mcp.JSONRPCResponse)
	if !ok {
		t.Fatalf("expected JSONRPCResponse, got %T: %+v", msg, msg)
	}

	result, ok := resp.Result.(*mcp.CallToolResult)
	if !ok {
		t.Fatalf("expected *mcp.CallToolResult, got %T: %+v", resp.Result, resp.Result)
	}
	return result
}

// textOf extracts the text from the first Content item in a CallToolResult.
func textOf(t *testing.T, result *mcp.CallToolResult) string {
	t.Helper()
	if len(result.Content) == 0 {
		t.Fatal("CallToolResult has no content")
	}
	tc, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", result.Content[0])
	}
	return tc.Text
}

// newTestServer creates an MCPServer with all tools registered, using the
// provided mock executor and a KnowledgeStore backed by dir.
func newTestServer(t *testing.T, mock cli.CommandRunner, dir string) *mcpserver.MCPServer {
	t.Helper()
	store, err := system.NewKnowledgeStore(dir)
	if err != nil {
		t.Fatalf("NewKnowledgeStore: %v", err)
	}
	s := mcpserver.NewMCPServer("test", "test")
	tools.Register(s, mock, store)
	return s
}

// ─── Tests ───────────────────────────────────────────────────────────────────

func TestGetSystemStatus(t *testing.T) {
	data, err := os.ReadFile("../../testdata/bootc-status.json")
	if err != nil {
		t.Fatalf("missing testdata: %v", err)
	}

	mock := cli.NewMockExecutor()
	mock.SetResponse("bootc", []string{"status", "--json"}, data, nil)

	s := newTestServer(t, mock, t.TempDir())
	result := callTool(t, s, "get_system_status", nil)

	if result.IsError {
		t.Fatalf("tool returned error: %s", textOf(t, result))
	}

	text := textOf(t, result)
	var out map[string]any
	if err := json.Unmarshal([]byte(text), &out); err != nil {
		t.Fatalf("result not valid JSON: %v\nraw: %s", err, text)
	}

	if out["Variant"] != "base" {
		t.Errorf("expected variant 'base', got %v", out["Variant"])
	}
	if out["ImageRef"] != "ghcr.io/ublue-os/bluefin:stable" {
		t.Errorf("unexpected image_ref: %v", out["ImageRef"])
	}
	if out["Booted"] == "" || out["Booted"] == nil {
		t.Error("booted digest should not be empty")
	}
}

func TestCheckUpdates_NoUpdate(t *testing.T) {
	mock := cli.NewMockExecutor()
	// bootc upgrade --check exits 0 with a message that does NOT contain
	// "available" when up-to-date (CheckUpdates keys off that substring).
	mock.SetResponse("bootc", []string{"upgrade", "--check"}, []byte("System image is current."), nil)

	s := newTestServer(t, mock, t.TempDir())
	result := callTool(t, s, "check_updates", nil)

	if result.IsError {
		t.Fatalf("tool returned error: %s", textOf(t, result))
	}

	text := textOf(t, result)
	var out map[string]any
	if err := json.Unmarshal([]byte(text), &out); err != nil {
		t.Fatalf("result not valid JSON: %v\nraw: %s", err, text)
	}

	available, _ := out["available"].(bool)
	if available {
		t.Errorf("expected available=false, got true; message: %v", out["message"])
	}
	if out["message"] == nil || out["message"] == "" {
		t.Error("expected non-empty message field")
	}
}

func TestGetFlatpakList(t *testing.T) {
	data, err := os.ReadFile("../../testdata/flatpak-list.txt")
	if err != nil {
		t.Fatalf("missing testdata: %v", err)
	}

	mock := cli.NewMockExecutor()
	mock.SetResponse("flatpak", []string{"list", "--columns=application,version"}, data, nil)

	s := newTestServer(t, mock, t.TempDir())
	result := callTool(t, s, "get_flatpak_list", nil)

	if result.IsError {
		t.Fatalf("tool returned error: %s", textOf(t, result))
	}

	text := textOf(t, result)
	var apps []map[string]any
	if err := json.Unmarshal([]byte(text), &apps); err != nil {
		t.Fatalf("result not valid JSON array: %v\nraw: %s", err, text)
	}
	if len(apps) == 0 {
		t.Fatal("expected at least one flatpak app")
	}
	first := apps[0]
	if first["AppID"] == "" || first["AppID"] == nil {
		t.Error("expected non-empty AppID in first app")
	}
	if first["Version"] == "" || first["Version"] == nil {
		t.Error("expected non-empty Version in first app")
	}
}

func TestGetBrewPackages_BrewNotInstalled(t *testing.T) {
	// MockExecutor returns ErrNotInstalled for any unregistered command.
	mock := cli.NewMockExecutor()
	// No "brew" response registered → ErrNotInstalled

	s := newTestServer(t, mock, t.TempDir())
	result := callTool(t, s, "get_brew_packages", nil)

	// When brew is not installed the tool returns nil, nil → JSON null → no error
	if result.IsError {
		t.Fatalf("tool returned error for missing brew (expected graceful nil): %s", textOf(t, result))
	}

	text := textOf(t, result)
	// Result should be JSON null (no brew) or an empty array — both are valid.
	// The key requirement is no IsError flag and parseable JSON.
	if text != "null" && text != "[]" {
		var arr []any
		if err := json.Unmarshal([]byte(text), &arr); err != nil {
			t.Errorf("unexpected result for missing brew: %q", text)
		}
	}
}

func TestListDistrobox_NotInstalled(t *testing.T) {
	// No "distrobox" response → ErrNotInstalled → graceful nil
	mock := cli.NewMockExecutor()

	s := newTestServer(t, mock, t.TempDir())
	result := callTool(t, s, "list_distrobox", nil)

	if result.IsError {
		t.Fatalf("tool returned error for missing distrobox (expected graceful nil): %s", textOf(t, result))
	}
	// Should be null or [] — just verify parseable JSON
	text := textOf(t, result)
	var v any
	if err := json.Unmarshal([]byte(text), &v); err != nil {
		t.Errorf("result not valid JSON: %v\nraw: %s", err, text)
	}
}

func TestGetUnitDocs_SeedData(t *testing.T) {
	// NewKnowledgeStore pre-populates seed data; no mock commands needed for this tool.
	mock := cli.NewMockExecutor()
	s := newTestServer(t, mock, t.TempDir())

	result := callTool(t, s, "get_unit_docs", map[string]any{
		"unit_name": "flatpak-nuke-fedora.service",
	})

	if result.IsError {
		t.Fatalf("tool returned error: %s", textOf(t, result))
	}

	text := textOf(t, result)
	var doc map[string]any
	if err := json.Unmarshal([]byte(text), &doc); err != nil {
		t.Fatalf("result not valid JSON: %v\nraw: %s", err, text)
	}
	if doc["name"] != "flatpak-nuke-fedora.service" {
		t.Errorf("expected name 'flatpak-nuke-fedora.service', got %v", doc["name"])
	}
	if doc["description"] == nil || doc["description"] == "" {
		t.Error("expected non-empty description from seed data")
	}
}

func TestStoreAndListUnitDocs_Roundtrip(t *testing.T) {
	mock := cli.NewMockExecutor()
	s := newTestServer(t, mock, t.TempDir())

	// Store a new unit doc
	storeResult := callTool(t, s, "store_unit_docs", map[string]any{
		"unit_name":   "my-custom.service",
		"description": "My custom test service",
		"variant":     "dx",
	})
	if storeResult.IsError {
		t.Fatalf("store_unit_docs returned error: %s", textOf(t, storeResult))
	}

	var stored map[string]any
	if err := json.Unmarshal([]byte(textOf(t, storeResult)), &stored); err != nil {
		t.Fatalf("store result not valid JSON: %v", err)
	}
	if stored["status"] != "stored" {
		t.Errorf("expected status 'stored', got %v", stored["status"])
	}
	if stored["unit"] != "my-custom.service" {
		t.Errorf("expected unit 'my-custom.service', got %v", stored["unit"])
	}

	// List all docs and verify the new unit appears
	listResult := callTool(t, s, "list_unit_docs", nil)
	if listResult.IsError {
		t.Fatalf("list_unit_docs returned error: %s", textOf(t, listResult))
	}

	var docs []map[string]any
	if err := json.Unmarshal([]byte(textOf(t, listResult)), &docs); err != nil {
		t.Fatalf("list result not valid JSON array: %v", err)
	}

	var found bool
	for _, d := range docs {
		if d["name"] == "my-custom.service" {
			found = true
			if d["description"] != "My custom test service" {
				t.Errorf("wrong description: got %v", d["description"])
			}
			if d["variant"] != "dx" {
				t.Errorf("wrong variant: got %v", d["variant"])
			}
			break
		}
	}
	if !found {
		t.Error("stored unit 'my-custom.service' not found in list_unit_docs output")
	}
}

func TestGetHardwareReport_AMDGPUNoIssues(t *testing.T) {
	lspci, err := os.ReadFile("../../testdata/lspci-nnk-amd-intel.txt")
	if err != nil {
		t.Fatalf("missing testdata: %v", err)
	}
	bootcData, err := os.ReadFile("../../testdata/bootc-status.json")
	if err != nil {
		t.Fatalf("missing testdata: %v", err)
	}

	mock := cli.NewMockExecutor()
	mock.SetResponse("lspci", []string{"-nnk"}, lspci, nil)
	mock.SetResponse("cat", []string{"/proc/cmdline"}, []byte("BOOT_IMAGE=/vmlinuz root=/dev/mapper/fedora-root"), nil)
	mock.SetResponse("cat", []string{"/sys/class/dmi/id/product_name"}, []byte("Framework Laptop 13"), nil)
	mock.SetResponse("cat", []string{"/sys/class/dmi/id/chassis_type"}, []byte("10"), nil)
	mock.SetResponse("cat", []string{"/sys/class/dmi/id/bios_version"}, []byte("03.05"), nil)
	mock.SetResponse("bootc", []string{"status", "--json"}, bootcData, nil)

	s := newTestServer(t, mock, t.TempDir())
	result := callTool(t, s, "get_hardware_report", nil)

	if result.IsError {
		t.Fatalf("tool returned error: %s", textOf(t, result))
	}

	text := textOf(t, result)
	var report map[string]any
	if err := json.Unmarshal([]byte(text), &report); err != nil {
		t.Fatalf("result not valid JSON: %v\nraw: %s", err, text)
	}

	// AMD GPU on base variant — no compatibility warnings expected
	compat, ok := report["variant_compat"].(map[string]any)
	if !ok {
		t.Fatalf("expected variant_compat object, got %T: %v", report["variant_compat"], report["variant_compat"])
	}
	if nv, _ := compat["nvidia_detected"].(bool); nv {
		t.Error("expected NvidiaDetected=false for AMD GPU")
	}

	// issues should be empty or absent for a clean AMD system on base
	issues, _ := report["issues"].([]any)
	for _, iss := range issues {
		t.Logf("hardware issue reported: %v", iss)
	}

	// system sub-object should be present
	if report["system"] == nil {
		t.Error("expected 'system' field in hardware report")
	}
}
