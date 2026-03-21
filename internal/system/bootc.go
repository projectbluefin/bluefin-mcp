package system

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/projectbluefin/bluefin-mcp/internal/cli"
)

// SystemStatus holds the current atomic OCI image state.
type SystemStatus struct {
	ImageRef          string `json:"image_ref"`
	Booted            string `json:"booted"` // digest
	Staged            string `json:"staged"` // digest if staged update present
	RollbackAvailable bool   `json:"rollback_available"`
	Variant           string `json:"variant"`
	Source            string `json:"source"` // "bootc" or "rpm-ostree"
}

// bootcStatusJSON matches `bootc status --json` output structure.
type bootcStatusJSON struct {
	Status struct {
		Booted *struct {
			Image struct {
				Image struct {
					Image string `json:"image"`
				} `json:"image"`
				ImageDigest string `json:"imageDigest"`
			} `json:"image"`
		} `json:"booted"`
		Staged *struct {
			Image struct {
				Image struct {
					Image string `json:"image"`
				} `json:"image"`
				ImageDigest string `json:"imageDigest"`
			} `json:"image"`
		} `json:"staged"`
		Rollback *struct{} `json:"rollback"`
	} `json:"status"`
}

type rpmOstreeJSON struct {
	Deployments []struct {
		Booted            bool   `json:"booted"`
		Checksum          string `json:"checksum"`
		Origin            string `json:"origin"`
		ContainerImageRef string `json:"container-image-reference"`
	} `json:"deployments"`
}

// GetSystemStatus queries bootc (or falls back to rpm-ostree) for the current image state.
func GetSystemStatus(ctx context.Context, runner cli.CommandRunner) (*SystemStatus, error) {
	// Try bootc first
	out, err := runner.Run(ctx, "bootc", []string{"status", "--json"})
	if err == nil {
		return parseBootcJSON(out)
	}
	// Fall back to rpm-ostree on any bootc failure (e.g. bootc requires root on some systems)
	out, err = runner.Run(ctx, "rpm-ostree", []string{"status", "--json"})
	if err != nil {
		return nil, err
	}
	return parseRpmOstreeJSON(out)
}

func parseBootcJSON(data []byte) (*SystemStatus, error) {
	var s bootcStatusJSON
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	st := &SystemStatus{Source: "bootc"}
	if s.Status.Booted != nil {
		st.ImageRef = s.Status.Booted.Image.Image.Image
		st.Booted = s.Status.Booted.Image.ImageDigest
	}
	if s.Status.Staged != nil {
		st.Staged = s.Status.Staged.Image.ImageDigest
	}
	st.RollbackAvailable = s.Status.Rollback != nil
	st.Variant = DetectVariant(st.ImageRef)
	return st, nil
}

func parseRpmOstreeJSON(data []byte) (*SystemStatus, error) {
	var r rpmOstreeJSON
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, err
	}
	st := &SystemStatus{Source: "rpm-ostree"}
	for _, d := range r.Deployments {
		if d.Booted {
			// Prefer container-image-reference (bootc-style); fall back to origin (legacy)
			ref := d.ContainerImageRef
			if ref == "" {
				ref = d.Origin
			}
			// Strip ostree transport prefixes
			ref = strings.TrimPrefix(ref, "ostree-unverified-registry:")
			ref = strings.TrimPrefix(ref, "ostree-image-signed:docker://")
			st.ImageRef = ref
			st.Booted = d.Checksum
			break
		}
	}
	st.Variant = DetectVariant(st.ImageRef)
	return st, nil
}

// DetectVariant extracts the Bluefin variant from an OCI image reference.
// Variants: base, dx, nvidia, aurora, aurora-dx
func DetectVariant(imageRef string) string {
	lower := strings.ToLower(imageRef)
	switch {
	case strings.Contains(lower, "aurora-dx"):
		return "aurora-dx"
	case strings.Contains(lower, "aurora"):
		return "aurora"
	case strings.Contains(lower, "bluefin-dx"):
		return "dx"
	case strings.Contains(lower, "bluefin-nvidia"):
		return "nvidia"
	default:
		return "base"
	}
}

// CheckUpdates checks if a Bluefin image update is available (non-blocking).
func CheckUpdates(ctx context.Context, runner cli.CommandRunner) (bool, string, error) {
	out, err := runner.Run(ctx, "bootc", []string{"upgrade", "--check"})
	if err != nil {
		if errors.Is(err, cli.ErrNotInstalled) {
			return false, "", nil
		}
		// bootc upgrade --check requires elevated privileges on some systems
		return false, "bootc requires elevated privileges to check for updates; run: sudo bootc upgrade --check", nil
	}
	text := string(out)
	available := strings.Contains(text, "Update available") || strings.Contains(text, "available")
	return available, text, nil
}
