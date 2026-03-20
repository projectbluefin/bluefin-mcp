package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/projectbluefin/bluefin-mcp/internal/cli"
	"github.com/projectbluefin/bluefin-mcp/internal/system"
)

// Register adds all 11 MCP tool handlers to the server.
func Register(s *server.MCPServer, runner cli.CommandRunner, store *system.KnowledgeStore) {
	s.AddTool(mcp.NewTool("get_system_status",
		mcp.WithDescription("Get atomic OCI image state: booted image, digest, staged update, variant"),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		st, err := system.GetSystemStatus(ctx, runner)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return jsonResult(st)
	})

	s.AddTool(mcp.NewTool("check_updates",
		mcp.WithDescription("Check if a Bluefin image update is available (non-blocking)"),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		available, msg, err := system.CheckUpdates(ctx, runner)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return jsonResult(map[string]any{"available": available, "message": msg})
	})

	s.AddTool(mcp.NewTool("get_boot_health",
		mcp.WithDescription("Get last boot health and rollback availability"),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		st, err := system.GetBootHealth(ctx, runner)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return jsonResult(map[string]any{
			"rollback_available": st.RollbackAvailable,
			"image_ref":          st.ImageRef,
			"variant":            st.Variant,
		})
	})

	s.AddTool(mcp.NewTool("get_variant_info",
		mcp.WithDescription("Detect the Bluefin variant (base, dx, nvidia, aurora, aurora-dx)"),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		st, err := system.GetSystemStatus(ctx, runner)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return jsonResult(map[string]string{"variant": st.Variant, "image_ref": st.ImageRef})
	})

	s.AddTool(mcp.NewTool("list_recipes",
		mcp.WithDescription("List available ujust recipes (Bluefin's automation surface)"),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		recipes, err := system.ListRecipes(ctx, runner)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return jsonResult(recipes)
	})

	s.AddTool(mcp.NewTool("get_flatpak_list",
		mcp.WithDescription("List installed Flatpak applications (Flathub-only on Bluefin)"),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		apps, err := system.GetFlatpakList(ctx, runner)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return jsonResult(apps)
	})

	s.AddTool(mcp.NewTool("get_brew_packages",
		mcp.WithDescription("List installed Homebrew CLI packages"),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		pkgs, err := system.GetBrewPackages(ctx, runner)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return jsonResult(pkgs)
	})

	s.AddTool(mcp.NewTool("list_distrobox",
		mcp.WithDescription("List Distrobox development containers"),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		boxes, err := system.ListDistrobox(ctx, runner)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return jsonResult(boxes)
	})

	s.AddTool(mcp.NewTool("get_unit_docs",
		mcp.WithDescription("Get semantic documentation for a Bluefin custom systemd unit"),
		mcp.WithString("unit_name", mcp.Required(), mcp.Description("Unit name, e.g. flatpak-nuke-fedora.service")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		name := req.GetString("unit_name", "")
		doc, err := store.GetUnitDoc(name)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return jsonResult(doc)
	})

	s.AddTool(mcp.NewTool("store_unit_docs",
		mcp.WithDescription("Store semantic documentation for a Bluefin custom systemd unit"),
		mcp.WithString("unit_name", mcp.Required(), mcp.Description("Unit name, e.g. my-custom.service")),
		mcp.WithString("description", mcp.Required(), mcp.Description("Human-readable description of what this unit does")),
		mcp.WithString("variant", mcp.Description("Which variant this applies to: all, dx, nvidia, aurora")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		name := req.GetString("unit_name", "")
		desc := req.GetString("description", "")
		variant := req.GetString("variant", "all")
		if variant == "" {
			variant = "all"
		}
		err := store.StoreUnitDoc(name, system.UnitDoc{
			Name: name, Description: desc, Variant: variant,
		})
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return jsonResult(map[string]string{"status": "stored", "unit": name})
	})

	s.AddTool(mcp.NewTool("list_unit_docs",
		mcp.WithDescription("List all documented Bluefin custom systemd units"),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		docs, err := store.ListUnitDocs()
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return jsonResult(docs)
	})

	// get_hardware_report complements linux-mcp-server, not replaces it.
	// Use linux-mcp-server for CPU, memory, disk, and full hardware inventory.
	// This tool adds: numeric PCI IDs + kernel modules (lspci -nnk), LiveCD
	// detection, laptop model from sysfs, and Bluefin variant compatibility.
	s.AddTool(mcp.NewTool("get_hardware_report",
		mcp.WithDescription("Hardware compatibility report for this Bluefin system. "+
			"Returns PCI devices with numeric vendor:device IDs and loaded kernel modules "+
			"(lspci -nnk — more detail than linux-mcp-server's get_hardware_information), "+
			"LiveCD detection, laptop model/chassis/firmware from sysfs (no root required), "+
			"and a Bluefin variant compatibility check. "+
			"Flags known-incompatible hardware: Broadcom WiFi (vendor 14e4, no wl driver in stock Fedora kernel) "+
			"and Nvidia GPU on a non-nvidia variant. "+
			"The static vendor list ships with the binary; it is not a live database. "+
			"For CPU details, RAM, disk, or a full hardware inventory, "+
			"use linux-mcp-server's get_cpu_information, get_memory_information, "+
			"get_disk_usage, and get_hardware_information instead."),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		report, err := system.GetHardwareReport(ctx, runner)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return jsonResult(report)
	})
}

func jsonResult(v any) (*mcp.CallToolResult, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("marshal error: %v", err)), nil
	}
	return mcp.NewToolResultText(string(data)), nil
}
