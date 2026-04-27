ALTER TABLE tasks
  ADD COLUMN requester_id BIGINT NULL AFTER creator_id,
  ADD KEY idx_tasks_requester_id (requester_id);

UPDATE tasks
SET requester_id = creator_id
WHERE requester_id IS NULL;
