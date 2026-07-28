import assert from "node:assert/strict";
import test from "node:test";

import {
  classifyCompatibilityConsoleEntries,
  classifyGuardAttempts,
  classifyGuardConsoleEntries,
  groupMatchesLocator,
  retiredActionsAbsent,
  validateSourceBundleManifest,
} from "./run_g7_playwright.mjs";

const ORIGIN = "http://127.0.0.1:18102";

function blockedConsole(url = "") {
  return {
    level: "error",
    text: "Failed to load resource: net::ERR_BLOCKED_BY_CLIENT.Inspector",
    url,
  };
}

test("locationless blocked console errors consume confirmed POST attempts once", () => {
  const attempts = classifyGuardAttempts(
    [
      { method: "POST", url: `${ORIGIN}/v1/auth/asset-cookie` },
      { method: "POST", url: `${ORIGIN}/v1/trace-events` },
      { method: "WEBSOCKET", url: "ws://127.0.0.1:18102/ws/v1" },
    ],
    ORIGIN,
  );
  const entries = classifyGuardConsoleEntries(
    [blockedConsole(), blockedConsole(), blockedConsole()],
    attempts.expected,
    ORIGIN,
  );
  assert.deepEqual(
    entries.map((entry) => entry.expected_guard_observation),
    [true, true, false],
  );
});

test("URL-bearing blocked errors require an exact unconsumed attempt", () => {
  const attempts = classifyGuardAttempts(
    [{ method: "POST", url: `${ORIGIN}/v1/auth/asset-cookie` }],
    ORIGIN,
  );
  const entries = classifyGuardConsoleEntries(
    [
      blockedConsole(`${ORIGIN}/v1/unknown-write`),
      blockedConsole(`${ORIGIN}/v1/auth/asset-cookie`),
      blockedConsole(`${ORIGIN}/v1/auth/asset-cookie`),
    ],
    attempts.expected,
    ORIGIN,
  );
  assert.deepEqual(
    entries.map((entry) => entry.expected_guard_observation),
    [false, true, false],
  );
});

test("over-limit guard attempts remain forbidden and unavailable to console matching", () => {
  const attempts = classifyGuardAttempts(
    [
      { method: "POST", url: `${ORIGIN}/v1/auth/asset-cookie` },
      { method: "POST", url: `${ORIGIN}/v1/auth/asset-cookie` },
      { method: "POST", url: `${ORIGIN}/v1/auth/asset-cookie` },
    ],
    ORIGIN,
  );
  assert.equal(attempts.expected.length, 2);
  assert.equal(attempts.forbidden.length, 1);
  assert.equal(attempts.forbidden[0].classification, "guard_count_exceeded");
  const entries = classifyGuardConsoleEntries(
    [blockedConsole(), blockedConsole(), blockedConsole()],
    attempts.expected,
    ORIGIN,
  );
  assert.deepEqual(
    entries.map((entry) => entry.expected_guard_observation),
    [true, true, false],
  );
});

test("V1 realtime reconnects are bounded and remain fail-closed", () => {
  const attempts = Array.from({ length: 9 }, () => ({
    method: "WEBSOCKET",
    url: "ws://127.0.0.1:18102/ws/v1",
  }));
  const classified = classifyGuardAttempts(attempts, ORIGIN);
  assert.equal(classified.expected.length, 8);
  assert.equal(classified.forbidden.length, 1);
  assert.equal(
    classified.forbidden[0].classification,
    "guard_count_exceeded",
  );
});

test("ordinary console errors are never guard observations", () => {
  const entries = classifyGuardConsoleEntries(
    [{ level: "error", text: "ordinary application failure", url: "" }],
    [{ method: "POST", url: `${ORIGIN}/v1/trace-events` }],
    ORIGIN,
  );
  assert.equal(entries[0].expected_guard_observation, false);
});

test("canonical group locators use the real resource-bundle scope fields", () => {
  assert.equal(
    groupMatchesLocator(
      {
        id: 4847,
        task_id: 2826,
        scope_kind: "sku",
        task_sku_item_id: 3076,
      },
      "group:2826:sku:3076",
      2826,
    ),
    true,
  );
  assert.equal(
    groupMatchesLocator(
      {
        id: 1264,
        task_id: 1264,
        scope_kind: "retouch_requirement",
        retouch_requirement_id: 45,
      },
      "group:1264:retouch_requirement:45",
      1264,
    ),
    true,
  );
  assert.equal(
    groupMatchesLocator(
      { id: 42, task_id: 900, scope_kind: "task" },
      "group:900:task:0",
      900,
    ),
    true,
  );
});

test("only confirmed retired-route 404s are approved transition observations", () => {
  const entries = classifyCompatibilityConsoleEntries(
    [
      {
        level: "error",
        text: "Failed to load resource: the server responded with a status of 404 (Not Found)",
        url: `${ORIGIN}/v1/tasks/2826/predictions?limit=5`,
      },
      {
        level: "error",
        text: "Failed to load resource: the server responded with a status of 404 (Not Found)",
        url: `${ORIGIN}/v1/tasks/2826/unknown`,
      },
    ],
    [
      {
        method: "GET",
        url: `${ORIGIN}/v1/tasks/2826/predictions`,
        status: 404,
      },
      {
        method: "GET",
        url: `${ORIGIN}/v1/tasks/2826/unknown`,
        status: 404,
      },
    ],
    "legacy_frontend_task_snapshot",
    ORIGIN,
    2826,
  );
  assert.equal(entries[0].expected_compatibility_observation, true);
  assert.equal(
    entries[0].expected_compatibility_route,
    "/v1/tasks/2826/predictions",
  );
  assert.equal(entries[1].expected_compatibility_observation, false);
});

test("transition 404s require the exact oracle kind and network confirmation", () => {
  const entry = {
    level: "error",
    text: "Failed to load resource: the server responded with a status of 404 (Not Found)",
    url: `${ORIGIN}/v1/tasks/2826/audit-supplements`,
  };
  assert.equal(
    classifyCompatibilityConsoleEntries(
      [entry],
      [],
      "legacy_frontend_task_snapshot",
      ORIGIN,
      2826,
    )[0].expected_compatibility_observation,
    false,
  );
  assert.equal(
    classifyCompatibilityConsoleEntries(
      [entry],
      [
        {
          method: "GET",
          url: `${ORIGIN}/v1/tasks/2826/audit-supplements`,
          status: 404,
        },
      ],
      "v8_resource_groups",
      ORIGIN,
      2826,
    )[0].expected_compatibility_observation,
    false,
  );
});

test("frontend rollback compatibility approves only the missing V8 bundle route", () => {
  const entry = {
    level: "error",
    text: "Failed to load resource: the server responded with a status of 404 (Not Found)",
    url: `${ORIGIN}/v1/tasks/2826/resource-bundle`,
  };
  const network = [
    {
      method: "GET",
      url: `${ORIGIN}/v1/tasks/2826/resource-bundle`,
      status: 404,
    },
  ];
  assert.equal(
    classifyCompatibilityConsoleEntries(
      [entry],
      network,
      "frontend_rollback_compatibility",
      ORIGIN,
      2826,
    )[0].expected_compatibility_observation,
    true,
  );
  assert.equal(
    classifyCompatibilityConsoleEntries(
      [{ ...entry, url: `${ORIGIN}/v1/tasks/2826/predictions` }],
      [
        {
          method: "GET",
          url: `${ORIGIN}/v1/tasks/2826/predictions`,
          status: 404,
        },
      ],
      "frontend_rollback_compatibility",
      ORIGIN,
      2826,
    )[0].expected_compatibility_observation,
    false,
  );
});

test("retired action assertion requires a complete clean DOM snapshot", () => {
  const clean = {
    actionControlSnapshotComplete: true,
    actionControls: [
      { text: "查看历史", action: "open_revision_history" },
      { text: "预览", action_key: "preview" },
    ],
    hook: { assertions: { retired_actions_absent: true } },
  };
  assert.equal(retiredActionsAbsent(clean, ["preview"]), true);
  assert.equal(retiredActionsAbsent({}, ["preview"]), false);
});

test("retired action assertion rejects visible text, action markers, and API actions", () => {
  const base = {
    actionControlSnapshotComplete: true,
    actionControls: [],
  };
  assert.equal(
    retiredActionsAbsent(
      { ...base, actionControls: [{ text: "仓库接收" }] },
      ["preview"],
    ),
    false,
  );
  assert.equal(
    retiredActionsAbsent(
      {
        ...base,
        actionControls: [{ text: "继续", action_key: "production_transfer" }],
      },
      ["preview"],
    ),
    false,
  );
  assert.equal(retiredActionsAbsent(base, ["pending_close"]), false);
});

test("source bundle manifest requires exact frozen order and member hashes", () => {
  const first = Buffer.from("first member");
  const second = Buffer.from("second member");
  const manifest = {
    version: 1,
    deterministic_profile: "zip-stored-fixed-1980-0644-v1",
    members: [
      {
        archive_path: "001_101_first.psd",
        task_asset_id: 101,
        confirmed: true,
        sha256:
          "9827136414f46014fd5f4e2e34453684a802ec3b5752e4e2919296ebb71e1d2e",
      },
      {
        archive_path: "002_102_second.psd",
        task_asset_id: 102,
        confirmed: true,
        sha256:
          "444d5b9f7a03d0847047e0600050207f5c37331b0e73d813118b60578fd911f4",
      },
    ],
  };
  const entries = {
    "manifest.json": Buffer.from(JSON.stringify(manifest)),
    "001_101_first.psd": first,
    "002_102_second.psd": second,
  };
  const expected = { ordered_member_task_asset_ids: [101, 102] };
  assert.equal(validateSourceBundleManifest(manifest, expected, entries), true);
  assert.equal(
    validateSourceBundleManifest(
      manifest,
      { ordered_member_task_asset_ids: [102, 101] },
      entries,
    ),
    false,
  );
  assert.equal(
    validateSourceBundleManifest(manifest, expected, {
      ...entries,
      "unexpected.txt": Buffer.from("unexpected"),
    }),
    false,
  );
  assert.equal(
    validateSourceBundleManifest(
      {
        ...manifest,
        members: [
          manifest.members[0],
          { ...manifest.members[1], confirmed: false },
        ],
      },
      expected,
      entries,
    ),
    false,
  );
});
