package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"workflow/domain"
)

// MediaECSProcessor shares the bounded ECS pool; legacy copies are read through
// the server OSS endpoint, never downloaded across the office WAN for proof.
type MediaECSProcessor struct {
	Previews         SystemMediaProcessor
	OSS              *OSSDirectService
	ExistingPreviews func(context.Context) (int, error)
}

func (p MediaECSProcessor) ProcessExistingPreviews(ctx context.Context) (int, error) {
	if p.ExistingPreviews == nil {
		return 0, nil
	}
	return p.ExistingPreviews(ctx)
}

func (p MediaECSProcessor) ProcessMediaPreview(ctx context.Context, job *domain.AssetMediaJob) error {
	if job.Kind != "external_copy_verify" {
		return p.Previews.ProcessMediaPreview(ctx, job)
	}
	if p.OSS == nil || !strings.Contains(p.OSS.cfg.Endpoint, "-internal.") {
		return fmt.Errorf("internal OSS endpoint required for legacy verification")
	}
	var input domain.AssetMediaJobInput
	if err := json.Unmarshal(job.Payload, &input); err != nil {
		return err
	}
	proof := input.Verification
	if proof == nil || proof.ObjectKey == "" || proof.ETag == "" || proof.Size < 0 {
		return fmt.Errorf("invalid legacy verification input")
	}
	before, exists, err := p.OSS.StatObject(ctx, proof.ObjectKey)
	if err != nil {
		return err
	}
	if !exists || before.ETag != proof.ETag || before.ContentLength != proof.Size {
		return fmt.Errorf("source_version_changed")
	}
	body, err := p.OSS.OpenObject(ctx, proof.ObjectKey)
	if err != nil {
		return err
	}
	defer body.Close()
	h := sha256.New()
	n, err := io.Copy(h, io.LimitReader(body, proof.Size+1))
	if err != nil {
		return err
	}
	after, exists, err := p.OSS.StatObject(ctx, proof.ObjectKey)
	if err != nil {
		return err
	}
	if !exists || before.ETag != after.ETag || after.ContentLength != proof.Size || n != proof.Size {
		return fmt.Errorf("source_version_changed")
	}
	job.Result, err = json.Marshal(struct {
		Matched bool `json:"matched"`
	}{Matched: hex.EncodeToString(h.Sum(nil)) == proof.SHA256})
	return err
}
