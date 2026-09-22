-- Complete bounded pricing models share the existing governed rule lifecycle.
-- Additive capacity change; historical expressions and prices remain unchanged.
ALTER TABLE cost_rules MODIFY COLUMN formula_expression TEXT NOT NULL;
