package risk

import (
	"context"
	"strings"

	"github.com/fortuna/core/pkg/models"
)

// attachScoreV3Preview adds a non-authoritative dimension snapshot to factors when the asset is a Pod
// and a row exists in asset_security_state (G-RE-01 bridge before full cut-over).
func (s *Scorer) attachScoreV3Preview(ctx context.Context, factors map[string]interface{}, resourceUID string, info ResourceInfoV2) {
	if factors == nil || s == nil || s.db == nil {
		return
	}
	if !strings.EqualFold(strings.TrimSpace(info.ResourceType), "Pod") {
		return
	}
	if strings.TrimSpace(resourceUID) == "" {
		return
	}
	var st models.AssetSecurityState
	if err := s.db.WithContext(ctx).Where("pod_uid = ?", resourceUID).First(&st).Error; err != nil {
		return
	}
	factors["score_v3_preview"] = buildScoreV3PreviewMap(&st)
}
