-- Migration: 110_users_employee_no.sql
-- Add a numeric employee number for stable user identity in user management.

ALTER TABLE users
  ADD COLUMN employee_no SMALLINT UNSIGNED NULL AFTER username,
  ADD UNIQUE KEY uq_users_employee_no (employee_no),
  ADD CONSTRAINT chk_users_employee_no_range CHECK (employee_no IS NULL OR employee_no BETWEEN 0 AND 9999);
