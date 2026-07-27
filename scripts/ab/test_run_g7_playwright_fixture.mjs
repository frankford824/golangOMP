import {
  buildPlan,
  executePlan,
  parseArgs,
} from "./run_g7_playwright.mjs";

const FIXTURE_ORIGINS = {
  external_external: "http://127.0.0.1:18101",
  devplus_devplus: "http://127.0.0.1:18102",
  external_devplus: "http://127.0.0.1:18103",
  devplus_external: "http://127.0.0.1:18104",
};

const identityRequests = {
  adminAuthMe: 0,
  deniedAuthMe: 0,
  deniedProfile: 0,
};

function json(body, status = 200) {
  return {
    status,
    contentType: "application/json; charset=utf-8",
    body: JSON.stringify(body),
  };
}

function bearerToken(requestOptions) {
  const authorization = String(requestOptions.headers?.authorization || "");
  return authorization.startsWith("Bearer ") ? authorization.slice(7) : "";
}

function fixtureApiResponse(url, status, contentType, body) {
  const payload = Buffer.from(body, "utf8");
  return {
    url: () => url,
    status: () => status,
    headers: () => ({ "content-type": contentType }),
    body: async () => payload,
    dispose: async () => {},
  };
}

async function identityRequester(_context, expectedUrl, requestOptions) {
  if (
    requestOptions.maxRedirects !== 0 ||
    requestOptions.failOnStatusCode !== false ||
    requestOptions.headers?.accept !== "application/json"
  ) {
    throw new Error("identity request did not preserve fail-closed options");
  }
  const url = new URL(expectedUrl);
  const combination = Object.entries(FIXTURE_ORIGINS).find(
    ([, origin]) => origin === url.origin,
  )?.[0];
  if (!combination || url.pathname !== "/v1/auth/me") {
    throw new Error("identity request used a non-authoritative endpoint");
  }
  const token = bearerToken(requestOptions);
  const isAdmin = token === `super-secret-cookie-admin-${combination}`;
  const isDenied =
    combination === "devplus_devplus" &&
    token === "super-secret-cookie-denied";
  if (!isAdmin && !isDenied) {
    return fixtureApiResponse(
      expectedUrl,
      401,
      "application/json; charset=utf-8",
      JSON.stringify({ code: "unauthenticated" }),
    );
  }
  if (process.env.G7_FIXTURE_AUTH_ME_REDIRECT === "1") {
    return fixtureApiResponse(
      expectedUrl,
      302,
      "application/json; charset=utf-8",
      JSON.stringify({ code: "redirect" }),
    );
  }
  if (process.env.G7_FIXTURE_AUTH_ME_HTML === "1") {
    return fixtureApiResponse(
      expectedUrl,
      200,
      "text/html; charset=utf-8",
      "<!doctype html><html><body>Login</body></html>",
    );
  }
  if (process.env.G7_FIXTURE_AUTH_ME_INVALID_JSON === "1") {
    return fixtureApiResponse(
      expectedUrl,
      200,
      "application/json; charset=utf-8",
      "{",
    );
  }
  if (isDenied) {
    identityRequests.deniedAuthMe += 1;
    return fixtureApiResponse(
      expectedUrl,
      200,
      "application/json; charset=utf-8",
      JSON.stringify({ data: { id: 339, roles: ["Member"] } }),
    );
  }
  identityRequests.adminAuthMe += 1;
  return fixtureApiResponse(
    expectedUrl,
    200,
    "application/json; charset=utf-8",
    JSON.stringify({
      data: {
        id: process.env.G7_FIXTURE_IDENTITY_MISMATCH === "1" ? 999 : 1,
        roles: ["Admin", "Designer", "Member", "Ops", "SuperAdmin"],
      },
    }),
  );
}

function fixtureHtml(
  scenario,
  {
    attemptPost,
    attemptServiceWorker,
    attemptWebSocket,
    attemptExpectedInfrastructure,
  },
) {
  return `<!doctype html>
<html>
<head>
  <meta charset="utf-8">
  <title>G7 ${scenario}</title>
  <style>
  html,body{margin:0;width:100%;height:100%} body{font-family:sans-serif}
  .task-detail-view{min-height:100vh;padding:20px;box-sizing:border-box}
  .revision-drawer{position:fixed;inset:20px;background:white;border:2px solid #333;padding:12px;overflow:auto}
  </style>
</head>
<body>
<main class="task-detail-view">
  <h1>Task 1</h1>
  <section class="resource-rail"><header><button type="button">Resources</button></header></section>
</main>
<script>
window.__G7_EVIDENCE__={
  assertions: {
    retired_actions_absent: true,
    terminal_actions_absent: true,
    bundle_members_match: true,
    no_cross_scope_assets: true,
    planning_fields_match: true,
    permission_denied_ui: true,
    historical_unavailable_ui: true,
    negative_state_rendered: true,
    approved_compatibility_difference_only: true
  },
  allowed_actions: { task_detail: ["preview"] },
  negative_state: true
};
const scenario = ${JSON.stringify(scenario)};
Promise.all([
  fetch("/v1/tasks/1?scenario=" + encodeURIComponent(scenario)).then(r => r.json()),
  fetch("/v1/tasks/1/detail?scenario=" + encodeURIComponent(scenario)).then(r => r.json()),
  fetch("/v1/tasks/1/resource-bundle?scenario=" + encodeURIComponent(scenario)).then(r => r.json())
]).then(() => document.body.dataset.ready = "true");
${attemptPost ? 'fetch("/forbidden-write", { method: "POST", body: "{}" }).catch(() => {});' : ""}
${attemptServiceWorker ? 'navigator.serviceWorker.register("/fixture-sw.js").catch(() => {});' : ""}
${attemptWebSocket ? 'try { new WebSocket("ws://127.0.0.1:18102/fixture-socket"); } catch {}' : ""}
${attemptExpectedInfrastructure ? `
fetch("/v1/auth/asset-cookie", { method: "POST" }).catch(() => {});
fetch("/v1/auth/asset-cookie", { method: "POST" }).catch(() => {});
fetch("/v1/trace-events", { method: "POST" }).catch(() => {});
try {
  new WebSocket((location.protocol === "https:" ? "wss://" : "ws://") + location.host + "/ws/v1");
} catch {}
` : ""}
document.querySelector(".resource-rail button").addEventListener("click", () => {
  if (document.querySelector(".workspace-dialog")) return;
  const workspace = document.createElement("section");
  workspace.className = "workspace-dialog";
  workspace.innerHTML = '<button class="revision-history-button" type="button">History</button>';
  document.body.appendChild(workspace);
  workspace.querySelector(".revision-history-button").addEventListener("click", async () => {
    const response = await fetch("/v1/resource-groups/10/revisions?page=1&page_size=20");
    await response.json();
    const drawer = document.createElement("aside");
    drawer.className = "revision-drawer";
    drawer.innerHTML =
      '<button aria-label="关闭历史修订" type="button">Close</button>' +
      '<article class="revision-card">' +
      '<dl class="revision-meta"><div><dt>Actor</dt><dd>Admin</dd></div><div><dt>Time</dt><dd>2026-07-23</dd></div></dl>' +
      '<section class="revision-files">Source</section>' +
      '<section class="revision-files">Final</section>' +
      '</article>' +
      '<button aria-label="下一页历史修订" type="button" disabled>Next</button>';
    document.body.appendChild(drawer);
    drawer.querySelector('[aria-label="关闭历史修订"]').addEventListener("click", () => drawer.remove());
  });
});
</script>
</body>
</html>`;
}

async function installContextRoutes(context, { role }) {
  await context.route("**/*", async (route) => {
    const request = route.request();
    const url = new URL(request.url());
    if (request.method() !== "GET") {
      await route.fulfill(json({ code: "method_not_allowed" }, 405));
      return;
    }
    if (url.pathname === "/probe/403") {
      if (
        role === "admin" &&
        process.env.G7_FIXTURE_ADMIN_403_REDIRECT === "1"
      ) {
        await route.fulfill({
          status: 302,
          headers: { location: "/fixture-login" },
          body: "",
        });
        return;
      }
      if (
        role === "admin" &&
        process.env.G7_FIXTURE_ADMIN_403_HTML === "1"
      ) {
        await route.fulfill({
          status: 200,
          contentType: "text/html; charset=utf-8",
          body: "<!doctype html><html><body>Login</body></html>",
        });
        return;
      }
      await route.fulfill(
        role === "denied"
          ? json({ code: "forbidden" }, 403)
          : json({ status: "authorized" }, 200),
      );
      return;
    }
    if (url.pathname === "/fixture-login") {
      await route.fulfill(json({ status: "login-page" }, 200));
      return;
    }
    if (url.pathname === "/v1/me") {
      const isAdmin = role === "admin";
      if (!isAdmin) identityRequests.deniedProfile += 1;
      await route.fulfill(
        isAdmin
          ? json({
              data: {
                id: 1,
                roles: ["Admin", "Designer", "Member", "Ops", "SuperAdmin"],
              },
            })
          : json({ code: "forbidden" }, 403),
      );
      return;
    }
    if (url.pathname === "/probe/410") {
      await route.fulfill(json({ code: "historical_unavailable" }, 410));
      return;
    }
    if (url.pathname === "/v1/tasks/1") {
      await route.fulfill(
        json({
          data: {
            id: 1,
            task_type: "sku_planning",
            allowed_actions: ["preview"],
            planning: { revision: 1 },
            access_token: "response-secret-token-must-not-survive",
          },
        }),
      );
      return;
    }
    if (url.pathname === "/v1/tasks/1/detail") {
      await route.fulfill(json({ data: { task: { id: 1 } } }));
      return;
    }
    if (url.pathname === "/v1/tasks/1/resource-bundle") {
      const groups =
        url.searchParams.get("scenario") === "missing_resource_group_negative"
          ? []
          : [
              {
                id: 10,
                task_id: 1,
                scope_kind: "sku",
                scope_ref_id: 1,
              },
            ];
      await route.fulfill(
        json({
          data: {
            groups,
          },
        }),
      );
      return;
    }
    if (url.pathname === "/v1/resource-groups/10/revisions") {
      await route.fulfill(
        json({
          data: {
            items: [
              {
                id: 1,
                status: "finalized",
                source_stage: "audit",
                created_by: 1,
                created_at: "2026-07-23T12:00:00Z",
                source_file: { task_asset_id: 1 },
                items: [{ id: 1, sort_order: 0 }],
                references: [],
              },
            ],
            page: 1,
            page_size: 20,
            total: 1,
          },
        }),
      );
      return;
    }
    if (url.pathname.startsWith("/tasks/1")) {
      const scenario = url.searchParams.get("g7_scenario") || "unknown";
      await route.fulfill({
        status: 200,
        contentType: "text/html; charset=utf-8",
        body: fixtureHtml(
          scenario,
          {
            attemptPost: process.env.G7_FIXTURE_ATTEMPT_POST === "1",
            attemptServiceWorker:
              process.env.G7_FIXTURE_ATTEMPT_SERVICE_WORKER === "1",
            attemptWebSocket:
              process.env.G7_FIXTURE_ATTEMPT_WEBSOCKET === "1",
            attemptExpectedInfrastructure:
              process.env.G7_FIXTURE_ATTEMPT_EXPECTED_INFRA === "1",
          },
        ),
      });
      return;
    }
    await route.fulfill(json({ code: "not_found" }, 404));
  });
}

const options = parseArgs(process.argv.slice(2));
if (options.mode !== "execute") {
  throw new Error("fixture harness only supports --execute");
}
const plan = await buildPlan(options);
const testHooks = { installContextRoutes, identityRequester };
if (process.env.G7_FIXTURE_FAIL_TRACE_SANITIZER === "1") {
  testHooks.traceSanitizer = async () => {
    throw new Error("fixture trace sanitizer failure");
  };
}
const evidence = await executePlan(plan, testHooks);
if (process.env.G7_FIXTURE_ASSERT_ZERO_ASSIGNMENT_SHAPE === "1") {
  if (
    identityRequests.adminAuthMe !== 8 ||
    identityRequests.deniedAuthMe !== 2 ||
    identityRequests.deniedProfile !== 0
  ) {
    throw new Error("zero-assignment identity endpoint shape was not preserved");
  }
}
process.stdout.write(
  `${JSON.stringify({ status: "PASS", record_count: evidence.records.length })}\n`,
);
