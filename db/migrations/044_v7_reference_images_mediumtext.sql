-- Migration: 044_v7_reference_images_mediumtext.sql
-- Increase task_details.reference_images_json capacity so the direct-upload
-- reference_images path can safely store up to 3 images at 3MB each.

ALTER TABLE task_details
  MODIFY COLUMN reference_images_json MEDIUMTEXT NOT NULL COMMENT 'JSON array of reference image payloads or URLs';
