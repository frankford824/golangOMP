-- Model edits append immutable rule rows. The historical seed identity omitted
-- rule_version, so an unchanged name/source could not be saved as version 2.
-- Index-only change: existing prices, formulas and bindings are not rewritten.
ALTER TABLE cost_rules
  DROP INDEX uq_cost_rules_seed_identity,
  ADD UNIQUE KEY uq_cost_rules_seed_identity
    (rule_name, category_code, rule_type, priority, source, special_process_keyword, rule_version);
