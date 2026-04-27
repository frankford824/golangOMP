-- Migration: 057_v1_0_org_master_convergence.sql
-- Purpose: converge org_departments / org_teams / users org references to the
-- v1.0 official master-data baseline. Legacy departments/teams are first
-- mapped to official targets on the users table, then the legacy rows are
-- disabled (not deleted) so historical foreign-key/audit integrity stays
-- intact while `/v1/org/options` stops exposing them.
--
-- The v1.0 official baseline is:
--   人事部       -> 人事管理组
--   运营部       -> 淘系一组, 淘系二组, 天猫一组, 天猫二组, 拼多多南京组, 拼多多池州组
--   设计研发部   -> 默认组
--   定制美工部   -> 默认组
--   审核部       -> 普通审核组, 定制审核组
--   云仓部       -> 默认组
--   未分配       -> 未分配池  (preserved system bucket)
--
-- All statements are written to be safely re-runnable: they only rewrite
-- rows that still carry a legacy value and they leave official rows alone.

-- -----------------------------------------------------------------------------
-- Step 1. Migrate active user rows off legacy departments/teams onto the
--         v1.0 official equivalents. The mapping choices are documented in
--         docs/V0_9_BACKEND_SOURCE_OF_TRUTH.md (Organization And Permission
--         Boundary Truth).
-- -----------------------------------------------------------------------------

-- 1a. Resolve legacy '设计部' user rows.
UPDATE users
   SET department = '定制美工部', team = '默认组'
 WHERE department = '设计部' AND team = '定制美工组';

UPDATE users
   SET department = '审核部', team = '普通审核组'
 WHERE department = '设计部' AND team = '设计审核组';

-- Any remaining 设计部 user (including 设计组 / 默认组 / null / unknown) folds
-- into the v1.0 design research and development department.
UPDATE users
   SET department = '设计研发部', team = '默认组'
 WHERE department = '设计部';

-- 1b. Resolve legacy '采购部' user rows into 运营部 (procurement is no longer a
--     standalone v1.0 department).
UPDATE users
   SET department = '运营部', team = '淘系一组'
 WHERE department = '采购部';

-- 1c. Resolve legacy '仓储部' and '烘焙仓储部' user rows into 云仓部.
UPDATE users
   SET department = '云仓部', team = '默认组'
 WHERE department IN ('仓储部', '烘焙仓储部');

-- 1d. Normalize legacy extra team names that still sit under official
--     departments.
UPDATE users SET team = '人事管理组'
 WHERE department = '人事部' AND team = '默认组';

UPDATE users SET team = '默认组'
 WHERE department = '设计研发部' AND team = '研发默认组';

UPDATE users SET team = '默认组'
 WHERE department = '定制美工部' AND team = '定制默认组';

UPDATE users SET team = '默认组'
 WHERE department = '云仓部' AND team = '云仓默认组';

UPDATE users SET team = '定制审核组'
 WHERE department = '审核部' AND team = '定制美工审核组';

UPDATE users SET team = '普通审核组'
 WHERE department = '审核部' AND team = '常规审核组';

-- 1e. Migrate legacy 运营一组..运营七组 into the v1.0 platform-aligned team
--     buckets on 运营部. The mapping is intentional: 淘系 for 运营1/2,
--     天猫 for 运营3/4, 拼多多南京 for 运营5, 拼多多池州 for 运营6/7.
UPDATE users SET team = '淘系一组'     WHERE department = '运营部' AND team = '运营一组';
UPDATE users SET team = '淘系二组'     WHERE department = '运营部' AND team = '运营二组';
UPDATE users SET team = '天猫一组'     WHERE department = '运营部' AND team = '运营三组';
UPDATE users SET team = '天猫二组'     WHERE department = '运营部' AND team = '运营四组';
UPDATE users SET team = '拼多多南京组' WHERE department = '运营部' AND team = '运营五组';
UPDATE users SET team = '拼多多池州组' WHERE department = '运营部' AND team = '运营六组';
UPDATE users SET team = '拼多多池州组' WHERE department = '运营部' AND team = '运营七组';

-- -----------------------------------------------------------------------------
-- Step 2. Disable legacy team rows in org_teams. Rows are preserved (not
--         deleted) to keep historical audit joins and migration reversibility
--         intact; disabled rows will not surface in `/v1/org/options` or in
--         validation selectors because both filters require enabled = 1.
-- -----------------------------------------------------------------------------

-- 2a. Disable legacy operations groups and department-level extra default
--     teams under the current official departments.
UPDATE org_teams
   SET enabled = 0
 WHERE name IN (
         '运营一组', '运营二组', '运营三组', '运营四组', '运营五组', '运营六组', '运营七组',
         '研发默认组', '定制默认组', '云仓默认组',
         '定制美工审核组', '常规审核组',
         '设计组', '定制美工组', '设计审核组',
         '采购组', '仓储组', '烘焙仓储组'
       );

-- 2b. Disable the 人事部 legacy '默认组' only; do not touch other '默认组'
--     rows that belong to 设计研发部 / 定制美工部 / 云仓部, which remain the
--     official Default Team for those departments.
UPDATE org_teams t
       INNER JOIN org_departments d ON d.id = t.department_id
   SET t.enabled = 0
 WHERE d.name = '人事部' AND t.name = '默认组';

-- -----------------------------------------------------------------------------
-- Step 3. Disable legacy department rows. Must run after step 1 so no active
--         user still points at a disabled department. Rows are preserved for
--         historical joins.
-- -----------------------------------------------------------------------------
UPDATE org_departments
   SET enabled = 0
 WHERE name IN ('设计部', '采购部', '仓储部', '烘焙仓储部');
