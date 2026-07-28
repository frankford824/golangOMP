import assert from "node:assert/strict";
import test from "node:test";

import {
  classifyGuardAttempts,
  classifyGuardConsoleEntries,
  groupMatchesLocator,
  retiredActionsAbsent,
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
