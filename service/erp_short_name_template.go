package service

import (
	"encoding/json"
	"os"
	"strings"
	"sync"
	"unicode/utf8"
)

const ERPProductShortNameMaxBytes = 40

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
	if maxLength > ERPProductShortNameMaxBytes {
		maxLength = ERPProductShortNameMaxBytes
	}
	return truncateERPShortName(shortName, maxLength)
}

func erpProductShortNameTooLong(value string) bool {
	return len(strings.TrimSpace(value)) > ERPProductShortNameMaxBytes
}

func erpProductShortNameForFiling(scene, templateKey, explicitShortName, name, iID string) string {
	explicitShortName = strings.TrimSpace(explicitShortName)
	if explicitShortName != "" {
		return truncateERPShortName(explicitShortName, ERPProductShortNameMaxBytes)
	}
	generated := generateERPShortName(scene, templateKey, name, iID)
	if strings.TrimSpace(generated) != "" {
		return truncateERPShortName(generated, ERPProductShortNameMaxBytes)
	}
	return truncateERPShortName(firstNonEmptyString(name, iID), ERPProductShortNameMaxBytes)
}

func truncateERPShortName(value string, maxBytes int) string {
	value = strings.TrimSpace(value)
	if maxBytes <= 0 {
		return ""
	}
	if len(value) <= maxBytes {
		return value
	}
	cut := 0
	for cut < len(value) {
		_, size := utf8.DecodeRuneInString(value[cut:])
		if size <= 0 || cut+size > maxBytes {
			break
		}
		cut += size
	}
	return strings.TrimSpace(value[:cut])
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
