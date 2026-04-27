-- Round S customization pricing bootstrap (v0.9 UAT).
-- Ownership: HRAdmin provides the final unit_price/weight_factor numbers.
-- DO NOT execute from this file. DBA will run it only after HRAdmin signs off.
-- Apply target: MAIN MySQL, table customization_pricing_rules.

-- Placeholders below must be replaced by HRAdmin before execution.
INSERT INTO customization_pricing_rules
  (customization_level_code, employment_type, unit_price, weight_factor, is_enabled)
VALUES
  ('L1', 'full_time', /* TBD by HRAdmin */ 0.00, 1.0000, 1),
  ('L1', 'part_time', /* TBD by HRAdmin */ 0.00, 1.0000, 1);
