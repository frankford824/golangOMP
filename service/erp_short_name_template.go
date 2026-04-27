package service

import (
	"encoding/json"
	"os"
	"strings"
	"sync"
)

type erpShortNameRuleConfig struct {
	Enabled         bool              `json:"enabled"`
	MaxLength       int               `json:"max_length"`
	DefaultTemplate string            `json:"default_template"`
	SceneTemplates  map[string]string `json:"scene_templates"`
	Templates       map[string]string `json:"templates"`
}

var (
	erpShortNameRuleOnce sync.Once
	erpShortNameRuleCfg  erpShortNameRuleConfig
)

func generateERPShortName(scene, templateKey, name, iID string) string {
	cfg := loadERPShortNameRuleConfig()
	if !cfg.Enabled {
		return ""
	}
	name = strings.TrimSpace(name)
	iID = strings.TrimSpace(iID)

	template := ""
	if templateKey != "" {
		template = strings.TrimSpace(cfg.Templates[strings.TrimSpace(templateKey)])
	}
	if template == "" {
		template = strings.TrimSpace(cfg.SceneTemplates[strings.TrimSpace(scene)])
	}
	if template == "" {
		template = strings.TrimSpace(cfg.DefaultTemplate)
	}
	if template == "" {
		template = "{name}-{i_id}"
	}

	shortName := strings.ReplaceAll(template, "{name}", name)
	shortName = strings.ReplaceAll(shortName, "{i_id}", iID)
	shortName = strings.TrimSpace(shortName)
	shortName = strings.Trim(shortName, "-_/")
	if shortName == "" {
		shortName = firstNonEmptyString(name, iID)
	}
	maxLength := cfg.MaxLength
	if maxLength <= 0 {
		maxLength = 32
	}
	if len(shortName) > maxLength {
		shortName = strings.TrimSpace(shortName[:maxLength])
	}
	return shortName
}

func loadERPShortNameRuleConfig() erpShortNameRuleConfig {
	erpShortNameRuleOnce.Do(func() {
		erpShortNameRuleCfg = erpShortNameRuleConfig{
			Enabled:         true,
			MaxLength:       32,
			DefaultTemplate: "{name}-{i_id}",
			SceneTemplates: map[string]string{
				"new_product_development": "{name}-{i_id}",
				"purchase_task":           "{name}-{i_id}",
				"original_product_update": "{name}-{i_id}",
				"item_style_update":       "{name}-{i_id}",
			},
			Templates: map[string]string{
				"default": "{name}-{i_id}",
			},
		}

		ruleFile := strings.TrimSpace(os.Getenv("ERP_SHORT_NAME_RULE_FILE"))
		if ruleFile == "" {
			ruleFile = "config/erp_short_name_rules.json"
		}
		raw, err := os.ReadFile(ruleFile)
		if err != nil || len(raw) == 0 {
			return
		}
		var external erpShortNameRuleConfig
		if err := json.Unmarshal(raw, &external); err != nil {
			return
		}
		if external.MaxLength > 0 {
			erpShortNameRuleCfg.MaxLength = external.MaxLength
		}
		if strings.TrimSpace(external.DefaultTemplate) != "" {
			erpShortNameRuleCfg.DefaultTemplate = strings.TrimSpace(external.DefaultTemplate)
		}
		erpShortNameRuleCfg.Enabled = external.Enabled
		if len(external.SceneTemplates) > 0 {
			erpShortNameRuleCfg.SceneTemplates = external.SceneTemplates
		}
		if len(external.Templates) > 0 {
			erpShortNameRuleCfg.Templates = external.Templates
		}
	})
	return erpShortNameRuleCfg
}
