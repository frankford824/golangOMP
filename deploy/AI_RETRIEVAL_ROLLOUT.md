# AI retrieval and data-assistant rollout

This rollout is additive and fail-closed. MySQL remains the business and authorization source of truth;
Qdrant contains only rebuildable vectors. Do not enable Chat or hybrid search before the shadow index has
passed the retrieval and permission golden set.

## Ordered rollout

1. Back up MySQL and apply migration `129_ai_chat_vector_retrieval.sql` while all new flags remain false.
2. Generate a random `QDRANT_API_KEY`, start `deploy/docker-compose.qdrant.yml`, and verify that port 6333 is
   bound only to loopback. Back up both the persistent volume and collection snapshots.
3. Configure the independent OpenAI-compatible embedding endpoint and an immutable
   `AI_EMBEDDING_VERSION` such as `text-embedding-v3:1024:20260719`.
4. Preview source counts without writes:

   ```bash
   go run ./cmd/tools/ai-retrieval-reindex --timeout 30m
   ```

5. Build a new versioned collection without switching the stable alias:

   ```bash
   AI_EMBEDDING_ENABLED=true VECTOR_SEARCH_ENABLED=true \
   go run ./cmd/tools/ai-retrieval-reindex \
     --apply --target-collection yongbo_retrieval_20260719_v1 --timeout 60m
   ```

6. Run the exact and semantic golden set, current-authorization leakage tests, and latency checks. Then rerun
   with `--switch-alias`; the tool creates a snapshot before atomically changing the stable alias.

   The reviewed golden file is environment evidence and must not contain credentials. Keep it outside the
   repository when it contains production identifiers. It must contain at least 50 mixed exact/hybrid cases:

   ```json
   {
     "cases": [
       {"id":"task-number-01","query":"RW-20260719-A-000001","scope":"tasks","mode":"exact","expected":["task:123"]},
       {"id":"semantic-reuse-01","query":"适合复用的夏季套装资源","scope":"assets","mode":"hybrid","expected":["task_resource_group:456"],"forbidden":["task_resource_group:999"]}
     ]
   }
   ```

   Run the evaluator with a least-privilege test user whose effective scope has been reviewed:

   ```bash
   AI_RETRIEVAL_EVAL_TOKEN='replace-at-runtime' \
   go run ./cmd/tools/ai-retrieval-eval --base-url http://127.0.0.1:8080 \
     --golden /secure/rehearsal/retrieval-golden.json
   ```

   The command fails unless exact Recall@1 is 100%, semantic Recall@10 is at least 95%, semantic NDCG@10
   is at least 0.80, and no response contains an entity listed in `forbidden`. Run separate reviewed files
   for self, department, team, selected-org and global scopes; a pass under one token does not prove another
   permission scope.
7. Enable only `VECTOR_SEARCH_ENABLED` and `AI_RETRIEVAL_WORKER_ENABLED`. Observe outbox backlog, retry
   alerts, Qdrant latency, permission rejections and exact-search regression for at least 24 hours.
8. Enable `AI_CHAT_ENABLED` for protected SuperAdmin canaries. Expand only to users with effective
   `report.view` after SSE, citation, owner-isolation, 429 and cancellation acceptance passes.
9. Finally use `/v1/search?mode=auto`. Rollback is immediate: disable `AI_CHAT_ENABLED` and
   `VECTOR_SEARCH_ENABLED`, or atomically point the alias at the previous collection.

Qdrant snapshots do not include aliases. Record the active alias mapping beside every snapshot and release
manifest. Never expose the Qdrant HTTP port publicly or log API keys, prompts, provider payloads, or raw
conversation bodies.

## Local pre-release rehearsal evidence (2026-07-20)

- A full local copy of the current production snapshot (2,048 tasks, 2,808 resource groups, 19,278 task
  assets and 218,422 external assets) accepted migration 129, its rollback block removed all seven tables
  owned by the migration, and the same forward migration then reapplied successfully. The rehearsal used
  fail-fast MySQL commands; it did not use `--force` or suppress SQL errors.
- The pinned `qdrant/qdrant:v1.18.2` container was bound to loopback and passed a real collection → stable
  alias → search → snapshot → delete lifecycle test.
- The full reindex integration projected a task and an external asset from MySQL, embedded both through a
  deterministic OpenAI-compatible test endpoint, indexed them into real Qdrant, switched the alias, and
  completed a second idempotent run with zero skipped tests.

These are local rehearsal results, not authorization to deploy and not substitutes for the 24-hour shadow
observation, scope-specific 50+ case golden sets, or a production backup/rollback rehearsal.
