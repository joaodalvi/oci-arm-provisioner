package provisioner

import (
	"context"
	"fmt"
	"time"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/core"
)

// VerifiedInstance contains the verification results for a launched instance.
type VerifiedInstance struct {
	InstanceID    string
	DisplayName   string
	PublicIP      string
	PrivateIP     string
	State         string
	Shape         string
	OCPUs         float32
	MemoryGB      float32
	Region        string
	Verified      bool
	SpecsMismatch bool
	Errors        []string
}

// Getter methods for logger interface compatibility
func (v *VerifiedInstance) GetInstanceID() string { return v.InstanceID }
func (v *VerifiedInstance) GetPublicIP() string   { return v.PublicIP }
func (v *VerifiedInstance) GetOCPUs() float32     { return v.OCPUs }
func (v *VerifiedInstance) GetMemoryGB() float32  { return v.MemoryGB }
func (v *VerifiedInstance) GetState() string      { return v.State }
func (v *VerifiedInstance) GetRegion() string     { return v.Region }

// VerifyInstance polls OCI to confirm the instance is RUNNING and specs match.
// It retrieves the public IP and validates the shape configuration.
func (w *AccountWorker) VerifyInstance(ctx context.Context, instanceID string) (*VerifiedInstance, error) {
	result := &VerifiedInstance{
		InstanceID: instanceID,
		Region:     w.Config.Region,
		Errors:     []string{},
	}

	// 1. Poll for RUNNING state (max 5 minutes, check every 10s)
	const maxWait = 5 * time.Minute
	const pollInterval = 10 * time.Second
	deadline := time.Now().Add(maxWait)

	w.Logger.Info(w.AccountName, "Verifying instance launch...")

	var instance *core.Instance
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		resp, err := w.ComputeClient.GetInstance(ctx, core.GetInstanceRequest{
			InstanceId: common.String(instanceID),
		})
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("GetInstance failed: %v", err))
			time.Sleep(pollInterval)
			continue
		}

		instance = &resp.Instance
		result.State = string(instance.LifecycleState)
		result.DisplayName = safeString(instance.DisplayName)
		result.Shape = safeString(instance.Shape)

		if instance.LifecycleState == core.InstanceLifecycleStateRunning {
			w.Logger.Info(w.AccountName, "Instance is RUNNING ✓")
			break
		}

		if instance.LifecycleState == core.InstanceLifecycleStateTerminated ||
			instance.LifecycleState == core.InstanceLifecycleStateTerminating {
			result.Errors = append(result.Errors, "Instance was terminated")
			return result, fmt.Errorf("instance terminated unexpectedly")
		}

		w.Logger.Info(w.AccountName, fmt.Sprintf("Instance state: %s (waiting for RUNNING...)", instance.LifecycleState))
		time.Sleep(pollInterval)
	}

	if instance == nil || instance.LifecycleState != core.InstanceLifecycleStateRunning {
		result.Errors = append(result.Errors, "Timeout waiting for RUNNING state")
		return result, fmt.Errorf("verification timeout: instance not running after %v", maxWait)
	}

	// 2. Verify Shape Configuration
	if instance.ShapeConfig != nil {
		if instance.ShapeConfig.Ocpus != nil {
			result.OCPUs = *instance.ShapeConfig.Ocpus
		}
		if instance.ShapeConfig.MemoryInGBs != nil {
			result.MemoryGB = *instance.ShapeConfig.MemoryInGBs
		}
	}

	// Check if specs match requested config
	if result.OCPUs != w.Config.OCPUs {
		result.SpecsMismatch = true
		result.Errors = append(result.Errors, fmt.Sprintf("OCPUs mismatch: requested %.1f, got %.1f", w.Config.OCPUs, result.OCPUs))
	}
	if result.MemoryGB != w.Config.MemoryGB {
		result.SpecsMismatch = true
		result.Errors = append(result.Errors, fmt.Sprintf("Memory mismatch: requested %.1fGB, got %.1fGB", w.Config.MemoryGB, result.MemoryGB))
	}

	if !result.SpecsMismatch {
		w.Logger.Info(w.AccountName, fmt.Sprintf("Specs verified: %.0f OCPUs, %.0fGB RAM ✓", result.OCPUs, result.MemoryGB))
	} else {
		w.Logger.Warn(w.AccountName, "Specs mismatch detected!")
	}

	// 3. Get VNIC Attachments to retrieve IP
	vnicResp, err := w.ComputeClient.ListVnicAttachments(ctx, core.ListVnicAttachmentsRequest{
		CompartmentId: common.String(w.Config.CompartmentOCID),
		InstanceId:    common.String(instanceID),
	})
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("ListVnicAttachments failed: %v", err))
		w.Logger.Warn(w.AccountName, fmt.Sprintf("Could not retrieve VNIC attachments: %v", err))
	} else if len(vnicResp.Items) > 0 {
		// Get the primary VNIC
		for _, att := range vnicResp.Items {
			if att.VnicId != nil && att.LifecycleState == core.VnicAttachmentLifecycleStateAttached {
				vnic, err := w.VirtualNetworkClient.GetVnic(ctx, core.GetVnicRequest{
					VnicId: att.VnicId,
				})
				if err != nil {
					result.Errors = append(result.Errors, fmt.Sprintf("GetVnic failed: %v", err))
					continue
				}
				if vnic.Vnic.PublicIp != nil {
					result.PublicIP = *vnic.Vnic.PublicIp
				}
				if vnic.Vnic.PrivateIp != nil {
					result.PrivateIP = *vnic.Vnic.PrivateIp
				}
				break // Got the primary VNIC
			}
		}
	}

	if result.PublicIP != "" {
		w.Logger.Info(w.AccountName, fmt.Sprintf("Public IP: %s ✓", result.PublicIP))
	} else {
		w.Logger.Warn(w.AccountName, "No public IP assigned (may take a moment)")
	}

	// Mark as verified if no critical errors
	result.Verified = len(result.Errors) == 0 || (len(result.Errors) > 0 && !result.SpecsMismatch && result.State == "RUNNING")

	return result, nil
}

// safeString safely dereferences a string pointer
func safeString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
