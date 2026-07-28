#!/usr/bin/env node

import crypto from "node:crypto";
import fs from "node:fs/promises";
import path from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";

const SCHEMA_VERSION = 1;
const GATE = "G7";
const SOURCE_KIND = "playwright";
const EXPECTED_SCENARIO_COUNT = 30;
const EXPECTED_CASE_COUNT = 66;
const EXPECTED_NO_RESOURCE_GROUP_SCENARIOS = new Set([
  "purchase_to_sku_planning",
]);
const COMBINATIONS = [
  "external_external",
  "devplus_devplus",
  "external_devplus",
  "devplus_external",
];
const VIEWPORTS = {
  desktop: { width: 1440, height: 900, device_scale_factor: 1 },
  mobile: { width: 390, height: 844, device_scale_factor: 1 },
};
const ORIGINS = {
  external_external: "http://127.0.0.1:18101",
  devplus_devplus: "http://127.0.0.1:18102",
  external_devplus: "http://127.0.0.1:18103",
  devplus_external: "http://127.0.0.1:18104",
};
const SHA256_RE = /^[0-9a-f]{64}$/;
const RUNTIME_GROUP_LOCATOR_RE = /^task_asset_group:([1-9][0-9]*)$/;
const CANONICAL_GROUP_LOCATOR_RE =
  /^group:([1-9][0-9]*):([A-Za-z0-9._-]+):([0-9]+)$/;
const SENSITIVE_KEY_RE =
  /^(authorization|proxy-authorization|cookie|set-cookie|cookies|storageState|storage_state|token|access_token|refresh_token|id_token|headers|requestHeaders|responseHeaders|postData)$/i;
const SENSITIVE_TEXT_RE =
  /(bearer\s+[A-Za-z0-9._~+/-]+=*|(?:access[_-]?token|refresh[_-]?token|id[_-]?token|token)\s*[:=]\s*\S+|authorization\s*[:=]\s*\S+|cookie\s*[:=]\s*\S+)/gi;
const MUTATING_METHODS = new Set(["POST", "PUT", "PATCH", "DELETE"]);
const READ_ONLY_METHODS = new Set(["GET", "HEAD", "OPTIONS"]);
const EXPECTED_GUARD_ATTEMPT_LIMITS = new Map([
  ["POST /v1/auth/asset-cookie", 2],
  ["POST /v1/trace-events", 2],
  // The fail-closed WebSocket route closes the V1 realtime connection before
  // any frame exchange. The frontend then follows its 1/2/4/8/16/30 second
  // reconnect schedule. Eight exact same-origin attempts cover a bounded
  // evidence case without treating that deterministic retry as a mutation;
  // the ninth still fails the gate as a reconnect storm.
  ["WEBSOCKET /ws/v1", 8],
]);
const RETIRED_ACTION_RE =
  /(?:仓库\s*接收|仓库\s*退回|生产\s*移交|待\s*结单|\bwarehouse(?:[_\s.-]*(?:receive|accept|reject|return|handoff))?\b|\bproduction[_\s.-]*transfer\b|\bpending[_\s.-]*close\b)/iu;

export class InputError extends Error {}

function isObject(value) {
  return value !== null && typeof value === "object" && !Array.isArray(value);
}

function nonempty(value) {
  return typeof value === "string" && value.trim() !== "";
}

function positiveInt(value) {
  return Number.isInteger(value) && value > 0;
}

function nonnegativeInt(value) {
  return Number.isInteger(value) && value >= 0;
}

function stableValue(value) {
  if (Array.isArray(value)) return value.map(stableValue);
  if (!isObject(value)) return value;
  return Object.fromEntries(
    Object.keys(value)
      .sort()
      .map((key) => [key, stableValue(value[key])]),
  );
}

export function canonicalJson(value) {
  return JSON.stringify(stableValue(value));
}

export function canonicalSha256(value) {
  return crypto.createHash("sha256").update(canonicalJson(value)).digest("hex");
}

async function fileSha256(filePath) {
  const hash = crypto.createHash("sha256");
  hash.update(await fs.readFile(filePath));
  return hash.digest("hex");
}

function textSha256(value) {
  return crypto.createHash("sha256").update(String(value)).digest("hex");
}

function bytesSha256(value) {
  return crypto.createHash("sha256").update(value).digest("hex");
}

function finishedAtFromMonotonic(startedEpochMs, startedMonotonicMs) {
  const elapsedMs = Math.max(0, performance.now() - startedMonotonicMs);
  return new Date(startedEpochMs + elapsedMs).toISOString();
}

async function readJson(filePath, label) {
  let parsed;
  try {
    parsed = JSON.parse(await fs.readFile(filePath, "utf8"));
  } catch (error) {
    throw new InputError(`${label} is not readable JSON: ${error.message}`);
  }
  if (!isObject(parsed)) throw new InputError(`${label} must be a JSON object`);
  return parsed;
}

function validateCanonicalHash(document, field, label) {
  const declared = document[field];
  const payload = { ...document };
  delete payload[field];
  if (!SHA256_RE.test(String(declared || "")) || canonicalSha256(payload) !== declared) {
    throw new InputError(`${label} has an invalid ${field}`);
  }
}

function assertExactKeys(value, expected, label) {
  if (!isObject(value)) throw new InputError(`${label} must be an object`);
  const actual = Object.keys(value).sort();
  const wanted = [...expected].sort();
  if (canonicalJson(actual) !== canonicalJson(wanted)) {
    throw new InputError(`${label} has an invalid shape`);
  }
}

function safeRelativeUrl(value, label) {
  if (!nonempty(value) || !value.startsWith("/") || value.startsWith("//")) {
    throw new InputError(`${label} must be a same-origin absolute path`);
  }
  const parsed = new URL(value, "http://127.0.0.1");
  if (
    parsed.origin !== "http://127.0.0.1" ||
    parsed.username ||
    parsed.password ||
    parsed.hash ||
    parsed.pathname.split("/").includes("..")
  ) {
    throw new InputError(`${label} is unsafe`);
  }
  for (const key of parsed.searchParams.keys()) {
    if (SENSITIVE_KEY_RE.test(key)) {
      throw new InputError(`${label} contains a sensitive query parameter`);
    }
  }
  return `${parsed.pathname}${parsed.search}`;
}

function requirementsFor(scenario, combination) {
  const conditional = scenario.requirements_by_combination;
  if (isObject(conditional) && isObject(conditional[combination])) {
    return conditional[combination];
  }
  return {
    requires_task_id: scenario.requires_task_id,
    requires_revision_ids: scenario.requires_revision_ids,
    requires_history_drawer: scenario.requires_history_drawer,
    required_http_statuses: scenario.required_http_statuses,
    required_assertions: scenario.required_assertions,
  };
}

function expectedAssertionsFor(scenarioId, combination, baseAssertions) {
  const expected = new Set(baseAssertions);
  if (
    scenarioId === "baseline_four_edge_readonly" &&
    combination === "devplus_external"
  ) {
    expected.add("approved_compatibility_difference_only");
  }
  return expected;
}

function validateRequirements(requirements, label) {
  assertExactKeys(
    requirements,
    [
      "requires_task_id",
      "requires_revision_ids",
      "requires_history_drawer",
      "required_http_statuses",
      "required_assertions",
    ],
    label,
  );
  for (const key of [
    "requires_task_id",
    "requires_revision_ids",
    "requires_history_drawer",
  ]) {
    if (typeof requirements[key] !== "boolean") {
      throw new InputError(`${label}.${key} must be boolean`);
    }
  }
  if (
    !Array.isArray(requirements.required_http_statuses) ||
    requirements.required_http_statuses.some(
      (status) => !Number.isInteger(status) || status < 100 || status > 599,
    ) ||
    new Set(requirements.required_http_statuses).size !==
      requirements.required_http_statuses.length
  ) {
    throw new InputError(`${label}.required_http_statuses is invalid`);
  }
  if (
    !Array.isArray(requirements.required_assertions) ||
    requirements.required_assertions.length === 0 ||
    requirements.required_assertions.some((name) => !nonempty(name)) ||
    new Set(requirements.required_assertions).size !==
      requirements.required_assertions.length
  ) {
    throw new InputError(`${label}.required_assertions is invalid`);
  }
}

function validateCatalog(catalog) {
  if (catalog.schema_version !== SCHEMA_VERSION || catalog.gate !== GATE) {
    throw new InputError("scenario catalog must be schema_version=1 and gate=G7");
  }
  if (
    !Array.isArray(catalog.combinations) ||
    canonicalJson([...catalog.combinations].sort()) !==
      canonicalJson([...COMBINATIONS].sort())
  ) {
    throw new InputError("scenario catalog must declare the exact four combinations");
  }
  if (
    !Array.isArray(catalog.viewports) ||
    canonicalJson([...catalog.viewports].sort()) !==
      canonicalJson(Object.keys(VIEWPORTS).sort())
  ) {
    throw new InputError("scenario catalog must declare desktop and mobile");
  }
  if (
    !Array.isArray(catalog.scenarios) ||
    catalog.scenarios.length !== EXPECTED_SCENARIO_COUNT
  ) {
    throw new InputError(
      `scenario catalog must contain exactly ${EXPECTED_SCENARIO_COUNT} G7 scenarios`,
    );
  }
  const seen = new Set();
  for (const scenario of catalog.scenarios) {
    if (
      !isObject(scenario) ||
      !nonempty(scenario.id) ||
      seen.has(scenario.id) ||
      scenario.critical !== true
    ) {
      throw new InputError("scenario catalog has an invalid or duplicate critical scenario");
    }
    seen.add(scenario.id);
    if (
      !Array.isArray(scenario.required_combinations) ||
      scenario.required_combinations.length === 0 ||
      scenario.required_combinations.some((value) => !COMBINATIONS.includes(value)) ||
      new Set(scenario.required_combinations).size !==
        scenario.required_combinations.length
    ) {
      throw new InputError(`scenario ${scenario.id} has invalid combinations`);
    }
    if (
      !Array.isArray(scenario.required_viewports) ||
      scenario.required_viewports.length === 0 ||
      scenario.required_viewports.some((value) => !(value in VIEWPORTS)) ||
      new Set(scenario.required_viewports).size !== scenario.required_viewports.length
    ) {
      throw new InputError(`scenario ${scenario.id} has invalid viewports`);
    }
    const baseRequirements = {
      requires_task_id: scenario.requires_task_id,
      requires_revision_ids: scenario.requires_revision_ids,
      requires_history_drawer: scenario.requires_history_drawer,
      required_http_statuses: scenario.required_http_statuses,
      required_assertions: scenario.required_assertions,
    };
    validateRequirements(baseRequirements, `scenario ${scenario.id}`);
    for (const combination of scenario.required_combinations) {
      const requirements = requirementsFor(scenario, combination);
      validateRequirements(requirements, `scenario ${scenario.id}/${combination}`);
      if (requirements.requires_task_id !== baseRequirements.requires_task_id) {
        throw new InputError(
          `scenario ${scenario.id}/${combination} cannot change task requirement`,
        );
      }
      if (
        canonicalJson([...requirements.required_http_statuses].sort()) !==
        canonicalJson([...baseRequirements.required_http_statuses].sort())
      ) {
        throw new InputError(
          `scenario ${scenario.id}/${combination} cannot weaken HTTP statuses`,
        );
      }
      if (
        canonicalJson([...requirements.required_assertions].sort()) !==
        canonicalJson(
          [...expectedAssertionsFor(
            scenario.id,
            combination,
            baseRequirements.required_assertions,
          )].sort(),
        )
      ) {
        throw new InputError(
          `scenario ${scenario.id}/${combination} cannot weaken assertions`,
        );
      }
    }
  }
  return catalog.scenarios;
}

function normalizeEdge(edge, combination, label) {
  assertExactKeys(
    edge,
    ["origin", "edge", "frontend_sha256", "backend_sha256", "fixture_identity"],
    label,
  );
  if (
    edge.origin !== ORIGINS[combination] ||
    edge.edge !== combination ||
    !SHA256_RE.test(String(edge.frontend_sha256 || "")) ||
    !SHA256_RE.test(String(edge.backend_sha256 || "")) ||
    !nonempty(edge.fixture_identity)
  ) {
    throw new InputError(`${label} does not match the fixed G7 edge identity`);
  }
  return { ...edge };
}

function normalizeReceipt(receipt, label) {
  if (
    receipt.schema_version !== SCHEMA_VERSION ||
    receipt.gate !== GATE ||
    receipt.status !== "PASS"
  ) {
    throw new InputError(`${label} must be a PASS schema_version=1 G7 receipt`);
  }
  validateCanonicalHash(receipt, "receipt_sha256", label);
  if (isObject(receipt.edges)) {
    if (
      canonicalJson(Object.keys(receipt.edges).sort()) !==
      canonicalJson([...COMBINATIONS].sort())
    ) {
      throw new InputError(`${label}.edges must contain exactly four combinations`);
    }
    return Object.fromEntries(
      COMBINATIONS.map((combination) => [
        combination,
        normalizeEdge(receipt.edges[combination], combination, `${label}.${combination}`),
      ]),
    );
  }
  if (!nonempty(receipt.combination) || !COMBINATIONS.includes(receipt.combination)) {
    throw new InputError(`${label} must contain edges or a valid combination`);
  }
  const edge = receipt.edge_identity || receipt.edge;
  return {
    [receipt.combination]: normalizeEdge(
      edge,
      receipt.combination,
      `${label}.${receipt.combination}`,
    ),
  };
}

function validateAllowedActions(rows, label) {
  if (!Array.isArray(rows) || rows.length === 0) {
    throw new InputError(`${label} must be a non-empty array`);
  }
  const seen = new Set();
  return rows
    .map((row, index) => {
      assertExactKeys(row, ["checkpoint", "expected"], `${label}[${index}]`);
      if (
        !nonempty(row.checkpoint) ||
        seen.has(row.checkpoint) ||
        !Array.isArray(row.expected) ||
        row.expected.some((action) => !nonempty(action)) ||
        new Set(row.expected).size !== row.expected.length
      ) {
        throw new InputError(`${label}[${index}] is invalid`);
      }
      seen.add(row.checkpoint);
      return { checkpoint: row.checkpoint, expected: [...row.expected].sort() };
    })
    .sort((left, right) => left.checkpoint.localeCompare(right.checkpoint));
}

function validateHttpProbes(rows, requiredStatuses, label) {
  if (!Array.isArray(rows)) throw new InputError(`${label} must be an array`);
  const seen = new Set();
  const normalized = rows.map((row, index) => {
    assertExactKeys(
      row,
      ["kind", "method", "path", "expected_status"],
      `${label}[${index}]`,
    );
    if (
      !nonempty(row.kind) ||
      seen.has(row.kind) ||
      row.method !== "GET" ||
      !Number.isInteger(row.expected_status) ||
      row.expected_status < 100 ||
      row.expected_status > 599
    ) {
      throw new InputError(`${label}[${index}] is invalid`);
    }
    seen.add(row.kind);
    return {
      kind: row.kind,
      method: "GET",
      path: safeRelativeUrl(row.path, `${label}[${index}].path`),
      expected_status: row.expected_status,
    };
  });
  if (
    canonicalJson(normalized.map((row) => row.expected_status).sort()) !==
    canonicalJson([...requiredStatuses].sort())
  ) {
    throw new InputError(`${label} does not exactly cover required HTTP statuses`);
  }
  return normalized.sort((left, right) => left.kind.localeCompare(right.kind));
}

function validateResourceOracle(value, combination, scenarioId, label) {
  if (!isObject(value)) {
    throw new InputError(`${label} must be an object`);
  }
  if (combination === "devplus_devplus") {
    assertExactKeys(value, ["kind"], label);
    const expectedKind =
      scenarioId === "missing_resource_group_negative"
        ? "v8_missing_resource_group"
        : EXPECTED_NO_RESOURCE_GROUP_SCENARIOS.has(scenarioId)
          ? "v8_expected_no_resource_groups"
        : "v8_resource_groups";
    if (value.kind !== expectedKind) {
      throw new InputError(`${label} must use the V8 resource-group oracle`);
    }
    return { kind: value.kind };
  }
  const expectedKinds = {
    external_external: "legacy_task_snapshot",
    external_devplus: "legacy_frontend_task_snapshot",
    devplus_external: "frontend_rollback_compatibility",
  };
  const keys =
    combination === "devplus_external"
      ? ["kind", "task_response_sha256", "approved_assertion"]
      : ["kind", "task_response_sha256"];
  assertExactKeys(value, keys, label);
  if (
    value.kind !== expectedKinds[combination] ||
    !SHA256_RE.test(String(value.task_response_sha256 || "")) ||
    (combination === "devplus_external" &&
      value.approved_assertion !== "approved_compatibility_difference_only")
  ) {
    throw new InputError(`${label} has invalid explicit compatibility semantics`);
  }
  return { ...value };
}

function sampleWithoutHash(sample) {
  const payload = { ...sample };
  delete payload.sample_sha256;
  return payload;
}

function validateSamples(samples, scenarios, catalogHash) {
  if (
    samples.schema_version !== SCHEMA_VERSION ||
    samples.gate !== GATE ||
    samples.status !== "PASS" ||
    samples.mode !== "final"
  ) {
    throw new InputError("samples manifest must be final PASS G7 schema_version=1");
  }
  if (
    !isObject(samples.input_sha256) ||
    samples.input_sha256.scenario_catalog_sha256 !== catalogHash
  ) {
    throw new InputError("samples manifest is not bound to the scenario catalog");
  }
  validateCanonicalHash(samples, "manifest_sha256", "samples manifest");
  if (
    !isObject(samples.sealed_edges) ||
    canonicalJson(Object.keys(samples.sealed_edges).sort()) !==
      canonicalJson([...COMBINATIONS].sort())
  ) {
    throw new InputError("samples manifest must seal all four edges");
  }
  const sealedEdges = Object.fromEntries(
    COMBINATIONS.map((combination) => [
      combination,
      normalizeEdge(
        samples.sealed_edges[combination],
        combination,
        `samples.sealed_edges.${combination}`,
      ),
    ]),
  );
  if (
    samples.scenario_count !== scenarios.length ||
    samples.sample_count !== scenarios.length ||
    !Array.isArray(samples.samples) ||
    samples.samples.length !== scenarios.length
  ) {
    throw new InputError(
      `samples manifest counts do not match the ${EXPECTED_SCENARIO_COUNT} scenarios`,
    );
  }
  const scenarioById = new Map(scenarios.map((scenario) => [scenario.id, scenario]));
  const cases = [];
  const sampleIds = new Set();
  for (const sample of samples.samples) {
    const scenario = scenarioById.get(sample?.scenario_id);
    if (
      !isObject(sample) ||
      !scenario ||
      sampleIds.has(sample.scenario_id) ||
      sample.status !== "READY" ||
      !SHA256_RE.test(String(sample.sample_sha256 || "")) ||
      canonicalSha256(sampleWithoutHash(sample)) !== sample.sample_sha256
    ) {
      throw new InputError("samples manifest contains an invalid READY sample");
    }
    sampleIds.add(sample.scenario_id);
    if (
      canonicalJson(sample.required_combinations) !==
        canonicalJson(scenario.required_combinations) ||
      canonicalJson(sample.required_viewports) !==
        canonicalJson(scenario.required_viewports) ||
      !Array.isArray(sample.coverage_matrix)
    ) {
      throw new InputError(`sample ${sample.scenario_id} does not match its scenario`);
    }
    const expectedKeys = new Set(
      scenario.required_combinations.flatMap((combination) =>
        scenario.required_viewports.map(
          (viewport) => `${sample.scenario_id}/${combination}/${viewport}`,
        ),
      ),
    );
    const actualKeys = new Set();
    for (const row of sample.coverage_matrix) {
      const key = `${sample.scenario_id}/${row?.combination}/${row?.viewport}`;
      if (!isObject(row) || !expectedKeys.has(key) || actualKeys.has(key)) {
        throw new InputError(`sample ${sample.scenario_id} has an unexpected case`);
      }
      actualKeys.add(key);
      const requirements = requirementsFor(scenario, row.combination);
      if (canonicalJson(row.requirements) !== canonicalJson(requirements)) {
        throw new InputError(`${key} requirement block drifted`);
      }
      if (
        (requirements.requires_task_id && !positiveInt(row.task_id)) ||
        (row.task_id !== null && !positiveInt(row.task_id)) ||
        !Array.isArray(row.revision_ids) ||
        row.revision_ids.some((value) => !positiveInt(value)) ||
        new Set(row.revision_ids).size !== row.revision_ids.length ||
        (requirements.requires_revision_ids && row.revision_ids.length === 0) ||
        !Array.isArray(row.resource_ids) ||
        row.resource_ids.some(
          (value) =>
            !nonempty(value) ||
            (!RUNTIME_GROUP_LOCATOR_RE.test(value) &&
              !CANONICAL_GROUP_LOCATOR_RE.test(value)),
        ) ||
        new Set(row.resource_ids).size !== row.resource_ids.length ||
        !SHA256_RE.test(String(row.oracle_sha256 || ""))
      ) {
        throw new InputError(`${key} has invalid sealed identities`);
      }
      const allowedActions = validateAllowedActions(
        row.allowed_actions,
        `${key}.allowed_actions`,
      );
      const probes = validateHttpProbes(
        row.http_probes,
        requirements.required_http_statuses,
        `${key}.http_probes`,
      );
      const resourceOracle = validateResourceOracle(
        row.resource_oracle,
        row.combination,
        sample.scenario_id,
        `${key}.resource_oracle`,
      );
      if (
        (resourceOracle.kind === "v8_resource_groups" &&
          row.resource_ids.length === 0) ||
        (![
          "v8_resource_groups",
          "v8_missing_resource_group",
          "v8_expected_no_resource_groups",
        ].includes(
          resourceOracle.kind,
        ) &&
          (row.resource_ids.length !== 0 || row.revision_ids.length !== 0))
        ||
        ([
          "v8_missing_resource_group",
          "v8_expected_no_resource_groups",
        ].includes(resourceOracle.kind) &&
          (row.resource_ids.length !== 0 || row.revision_ids.length !== 0))
      ) {
        throw new InputError(`${key} resource identities contradict its edge oracle`);
      }
      cases.push({
        key,
        scenario,
        sample,
        coverage: {
          ...row,
          allowed_actions: allowedActions,
          http_probes: probes,
          resource_oracle: resourceOracle,
        },
      });
    }
    if (
      actualKeys.size !== expectedKeys.size ||
      [...expectedKeys].some((key) => !actualKeys.has(key))
    ) {
      throw new InputError(`sample ${sample.scenario_id} has incomplete coverage`);
    }
  }
  if (
    sampleIds.size !== scenarios.length ||
    cases.length !== EXPECTED_CASE_COUNT
  ) {
    throw new InputError(
      `hardened samples must resolve exactly ${EXPECTED_CASE_COUNT} G7 cases`,
    );
  }
  return { sealedEdges, cases };
}

function validateApiOracle(apiOracle, samples, cases, catalogHash) {
  if (
    apiOracle.schema_version !== SCHEMA_VERSION ||
    apiOracle.gate !== GATE ||
    apiOracle.status !== "PASS" ||
    apiOracle.source_kind !== "reviewed_api_allowed_actions" ||
    !positiveInt(apiOracle.reviewed_by) ||
    !nonempty(apiOracle.reviewed_at) ||
    !nonempty(apiOracle.review_note)
  ) {
    throw new InputError("API oracle is not a reviewed PASS G7 oracle");
  }
  validateCanonicalHash(apiOracle, "manifest_sha256", "API oracle");
  const expectedOracleInputs = {
    scenario_catalog_sha256: catalogHash,
    mapping_sha256: samples.input_sha256.mapping_sha256,
    canonical_entities_sha256:
      samples.input_sha256.canonical_entities_sha256,
    edge_receipt_sha256: samples.input_sha256.edge_receipt_sha256,
    fixture_receipt_sha256: samples.input_sha256.fixture_receipt_sha256,
  };
  if (canonicalJson(apiOracle.input_sha256) !== canonicalJson(expectedOracleInputs)) {
    throw new InputError("API oracle is not bound to the samples upstream inputs");
  }
  if (!Array.isArray(apiOracle.cases)) {
    throw new InputError("API oracle cases must be an array");
  }
  const indexed = new Map();
  for (const row of apiOracle.cases) {
    if (
      !isObject(row) ||
      !nonempty(row.scenario_id) ||
      !COMBINATIONS.includes(row.combination)
    ) {
      throw new InputError("API oracle contains an invalid case identity");
    }
    const key = `${row.scenario_id}/${row.combination}`;
    if (indexed.has(key)) throw new InputError("API oracle contains duplicate cases");
    indexed.set(key, {
      allowed_actions: validateAllowedActions(
        row.allowed_actions,
        `API oracle ${key}.allowed_actions`,
      ),
      http_probes: validateHttpProbes(
        row.http_probes,
        cases.find(
          (item) =>
            item.scenario.id === row.scenario_id &&
            item.coverage.combination === row.combination,
        )?.coverage.requirements.required_http_statuses || [],
        `API oracle ${key}.http_probes`,
      ),
      resource_oracle: validateResourceOracle(
        row.resource_oracle,
        row.combination,
        row.scenario_id,
        `API oracle ${key}.resource_oracle`,
      ),
    });
  }
  const expectedKeys = new Set(
    cases.map((item) => `${item.scenario.id}/${item.coverage.combination}`),
  );
  if (
    indexed.size !== expectedKeys.size ||
    [...expectedKeys].some((key) => !indexed.has(key))
  ) {
    throw new InputError("API oracle does not cover every scenario/combination");
  }
  for (const item of cases) {
    const key = `${item.scenario.id}/${item.coverage.combination}`;
    const oracle = indexed.get(key);
    if (
      canonicalJson(oracle.allowed_actions) !==
        canonicalJson(item.coverage.allowed_actions) ||
      canonicalJson(oracle.http_probes) !==
        canonicalJson(item.coverage.http_probes) ||
      canonicalJson(oracle.resource_oracle) !==
        canonicalJson(item.coverage.resource_oracle)
    ) {
      throw new InputError(`${key} differs from the reviewed API oracle`);
    }
  }
}

function validateFixtureReceipt(fixtureReceipt) {
  if (
    fixtureReceipt.schema_version !== 2 ||
    fixtureReceipt.gate !== GATE ||
    fixtureReceipt.status !== "APPLIED_VERIFIED_PENDING_UI_AND_CLEANUP" ||
    !nonempty(fixtureReceipt.run_id)
  ) {
    throw new InputError("fixture receipt is not the verified G7 v2 receipt");
  }
  validateCanonicalHash(
    fixtureReceipt,
    "receipt_payload_sha256",
    "fixture receipt",
  );
}

async function validateUpstreamBindings({
  samples,
  cases,
  catalogHash,
  apiOraclePath,
  fixtureReceiptPath,
  receiptPaths,
  receipts,
}) {
  assertExactKeys(
    samples.input_sha256,
    [
      "scenario_catalog_sha256",
      "mapping_sha256",
      "canonical_entities_sha256",
      "edge_receipt_sha256",
      "fixture_receipt_sha256",
      "api_oracle_sha256",
    ],
    "samples.input_sha256",
  );
  for (const [name, digest] of Object.entries(samples.input_sha256)) {
    if (!SHA256_RE.test(String(digest || ""))) {
      throw new InputError(`samples.input_sha256.${name} must be SHA-256`);
    }
  }
  assertExactKeys(
    samples.oracle_contract,
    [
      "kind",
      "edge_receipt_manifest_sha256",
      "api_oracle_manifest_sha256",
      "fixture_receipt_payload_sha256",
      "executor_supplied_oracle_forbidden",
    ],
    "samples.oracle_contract",
  );
  if (
    samples.oracle_contract.kind !== "reviewed_api_allowed_actions_v1" ||
    samples.oracle_contract.executor_supplied_oracle_forbidden !== true ||
    !SHA256_RE.test(samples.oracle_contract.edge_receipt_manifest_sha256 || "") ||
    !SHA256_RE.test(samples.oracle_contract.api_oracle_manifest_sha256 || "") ||
    !SHA256_RE.test(samples.oracle_contract.fixture_receipt_payload_sha256 || "")
  ) {
    throw new InputError("samples.oracle_contract is invalid");
  }
  if (receiptPaths.length !== 1 || !receipts[0]?.document?.edges) {
    throw new InputError(
      "upstream binding requires one self-hashed receipt containing all four edges",
    );
  }
  if (
    receipts[0].fileSha256 !== samples.input_sha256.edge_receipt_sha256 ||
    receipts[0].document.receipt_sha256 !==
      samples.oracle_contract.edge_receipt_manifest_sha256
  ) {
    throw new InputError("edge receipt is not bound to the samples oracle contract");
  }
  await assertRegularFile(apiOraclePath, "API oracle");
  const apiOracle = await readJson(apiOraclePath, "API oracle");
  validateApiOracle(apiOracle, samples, cases, catalogHash);
  if (
    (await fileSha256(apiOraclePath)) !== samples.input_sha256.api_oracle_sha256 ||
    apiOracle.manifest_sha256 !==
      samples.oracle_contract.api_oracle_manifest_sha256
  ) {
    throw new InputError("API oracle hashes do not match the samples contract");
  }
  await assertRegularFile(fixtureReceiptPath, "fixture receipt");
  const fixtureReceipt = await readJson(fixtureReceiptPath, "fixture receipt");
  validateFixtureReceipt(fixtureReceipt);
  if (
    (await fileSha256(fixtureReceiptPath)) !==
      samples.input_sha256.fixture_receipt_sha256 ||
    fixtureReceipt.receipt_payload_sha256 !==
      samples.oracle_contract.fixture_receipt_payload_sha256
  ) {
    throw new InputError("fixture receipt hashes do not match the samples contract");
  }
  return {
    apiOracleManifestSha256: apiOracle.manifest_sha256,
    fixtureReceiptPayloadSha256: fixtureReceipt.receipt_payload_sha256,
    edgeReceiptSha256: receipts[0].document.receipt_sha256,
  };
}

async function assertRegularFile(filePath, label) {
  let stats;
  try {
    stats = await fs.lstat(filePath);
  } catch {
    throw new InputError(`${label} does not exist`);
  }
  if (!stats.isFile() || stats.isSymbolicLink()) {
    throw new InputError(`${label} must be a regular non-symlink file`);
  }
}

async function assertSecretPath(filePath, secretRoot, label) {
  await assertRegularFile(filePath, label);
  const resolved = await fs.realpath(filePath);
  const root = await fs.realpath(secretRoot);
  const relative = path.relative(root, resolved);
  if (relative === "" || relative.startsWith("..") || path.isAbsolute(relative)) {
    throw new InputError(`${label} must be below the run-scoped secret root`);
  }
  const mode = (await fs.stat(resolved)).mode & 0o777;
  if (process.platform !== "win32" && (mode & 0o077) !== 0) {
    throw new InputError(`${label} must not be group/world accessible`);
  }
  return resolved;
}

async function storageStateMetadata(filePath, label, expectedOrigin) {
  const document = await readJson(filePath, label);
  if (!Array.isArray(document.cookies) || !Array.isArray(document.origins)) {
    throw new InputError(`${label} must contain Playwright cookies and origins arrays`);
  }
  const sensitiveValues = [];
  for (const cookie of document.cookies) {
    if (!isObject(cookie) || !nonempty(cookie.name) || typeof cookie.value !== "string") {
      throw new InputError(`${label} contains an invalid cookie`);
    }
    if (cookie.value.length >= 4) sensitiveValues.push(cookie.value);
  }
  for (const origin of document.origins) {
    if (!isObject(origin) || !Array.isArray(origin.localStorage)) continue;
    for (const entry of origin.localStorage) {
      if (
        isObject(entry) &&
        nonempty(entry.name) &&
        typeof entry.value === "string" &&
        entry.value.length >= 4
      ) {
        sensitiveValues.push(entry.value);
      }
    }
  }
  const matchingOrigins = document.origins.filter(
    (origin) => isObject(origin) && origin.origin === expectedOrigin,
  );
  if (matchingOrigins.length !== 1) {
    throw new InputError(`${label} must contain exactly one attested origin`);
  }
  if (!Array.isArray(matchingOrigins[0].localStorage)) {
    throw new InputError(`${label} attested origin must contain localStorage`);
  }
  const accessTokens = matchingOrigins[0].localStorage.filter(
    (entry) =>
      isObject(entry) &&
      entry.name === "access_token" &&
      typeof entry.value === "string" &&
      entry.value.trim() !== "",
  );
  if (accessTokens.length !== 1) {
    throw new InputError(`${label} must contain exactly one localStorage access_token`);
  }
  return {
    fileSha256: await fileSha256(filePath),
    sensitiveValues: [...new Set(sensitiveValues)],
    accessToken: accessTokens[0].value,
  };
}

async function validateAuthAttestation({
  attestationPath,
  runId,
  secretRoot,
  adminStates,
  deniedState,
}) {
  await assertRegularFile(attestationPath, "auth attestation");
  const attestation = await readJson(attestationPath, "auth attestation");
  if (
    attestation.schema_version !== SCHEMA_VERSION ||
    attestation.gate !== GATE ||
    attestation.status !== "PASS" ||
    attestation.run_id !== runId
  ) {
    throw new InputError("auth attestation is not a PASS document for this run_id");
  }
  validateCanonicalHash(attestation, "attestation_sha256", "auth attestation");
  const realSecretRoot = await fs.realpath(secretRoot);
  if (
    path.basename(realSecretRoot) !== runId ||
    attestation.secret_root_sha256 !== textSha256(realSecretRoot)
  ) {
    throw new InputError("secret root is not cryptographically bound to this run_id");
  }
  assertExactKeys(attestation.states, ["admin", "denied"], "auth attestation.states");
  assertExactKeys(
    attestation.states.admin,
    COMBINATIONS,
    "auth attestation.states.admin",
  );
  const sensitiveValues = [];
  const adminHashes = new Set();
  const adminIdentityIds = new Set();
  const runtimeExpectations = { admin: {}, denied: null };
  const runtimeTokens = { admin: {}, denied: null };
  for (const combination of COMBINATIONS) {
    const row = attestation.states.admin[combination];
    assertExactKeys(
      row,
      ["identity_id", "role", "origin", "storage_state_sha256"],
      `auth attestation admin ${combination}`,
    );
    const metadata = await storageStateMetadata(
      adminStates[combination],
      `admin auth state ${combination}`,
      ORIGINS[combination],
    );
    if (
      !nonempty(row.identity_id) ||
      row.role !== "Admin" ||
      row.origin !== ORIGINS[combination] ||
      row.storage_state_sha256 !== metadata.fileSha256
    ) {
      throw new InputError(`admin auth attestation mismatch for ${combination}`);
    }
    adminHashes.add(metadata.fileSha256);
    adminIdentityIds.add(row.identity_id);
    sensitiveValues.push(...metadata.sensitiveValues);
    runtimeExpectations.admin[combination] = {
      identity_id: row.identity_id,
      role: row.role,
      origin: row.origin,
    };
    runtimeTokens.admin[combination] = metadata.accessToken;
  }
  const denied = attestation.states.denied;
  assertExactKeys(
    denied,
    [
      "identity_id",
      "role",
      "origin",
      "combination",
      "storage_state_sha256",
    ],
    "auth attestation denied",
  );
  const deniedMetadata = await storageStateMetadata(
    deniedState,
    "Clone B denied auth state",
    ORIGINS.devplus_devplus,
  );
  if (
    !nonempty(denied.identity_id) ||
    !nonempty(denied.role) ||
    denied.role === "Admin" ||
    denied.combination !== "devplus_devplus" ||
    denied.origin !== ORIGINS.devplus_devplus ||
    denied.storage_state_sha256 !== deniedMetadata.fileSha256 ||
    adminHashes.has(deniedMetadata.fileSha256) ||
    adminIdentityIds.has(denied.identity_id)
  ) {
    throw new InputError("denied auth identity is not independent from admin");
  }
  sensitiveValues.push(...deniedMetadata.sensitiveValues);
  runtimeExpectations.denied = {
    identity_id: denied.identity_id,
    role: denied.role,
    origin: denied.origin,
  };
  runtimeTokens.denied = deniedMetadata.accessToken;
  return {
    attestationSha256: attestation.attestation_sha256,
    sensitiveValues: [...new Set(sensitiveValues)],
    runtimeExpectations,
    runtimeTokens,
  };
}

async function loadReceipts(receiptPaths) {
  if (!Array.isArray(receiptPaths) || receiptPaths.length === 0) {
    throw new InputError("at least one --edge-receipt is required");
  }
  const edges = {};
  const receipts = [];
  for (const [index, receiptPath] of receiptPaths.entries()) {
    await assertRegularFile(receiptPath, `edge receipt ${index + 1}`);
    const document = await readJson(receiptPath, `edge receipt ${index + 1}`);
    const normalized = normalizeReceipt(
      document,
      `edge receipt ${index + 1}`,
    );
    receipts.push({
      document,
      fileSha256: await fileSha256(receiptPath),
    });
    for (const [combination, edge] of Object.entries(normalized)) {
      if (edges[combination]) {
        throw new InputError(`duplicate receipt for ${combination}`);
      }
      edges[combination] = edge;
    }
  }
  if (
    canonicalJson(Object.keys(edges).sort()) !==
    canonicalJson([...COMBINATIONS].sort())
  ) {
    throw new InputError("edge receipts do not resolve exactly four combinations");
  }
  return { edges, receipts };
}

function sameEdge(left, right) {
  return canonicalJson(left) === canonicalJson(right);
}

export async function buildPlan(options) {
  const catalogPath = path.resolve(options.scenarios);
  const samplesPath = path.resolve(options.samples);
  const secretRoot = path.resolve(options.secretRoot);
  const artifactRoot = path.resolve(options.artifactRoot);
  const outputPath = path.resolve(options.output);
  if (!nonempty(options.runId) || !/^[A-Za-z0-9._-]+$/.test(options.runId)) {
    throw new InputError("--run-id must be a filesystem-safe identifier");
  }
  await assertRegularFile(catalogPath, "scenario catalog");
  await assertRegularFile(samplesPath, "samples manifest");
  const catalog = await readJson(catalogPath, "scenario catalog");
  const scenarios = validateCatalog(catalog);
  const catalogHash = await fileSha256(catalogPath);
  const samples = await readJson(samplesPath, "samples manifest");
  const { sealedEdges, cases } = validateSamples(samples, scenarios, catalogHash);
  const { edges: receiptEdges, receipts } = await loadReceipts(options.edgeReceipts);
  for (const combination of COMBINATIONS) {
    if (!sameEdge(receiptEdges[combination], sealedEdges[combination])) {
      throw new InputError(`${combination} receipt differs from samples.sealed_edges`);
    }
  }
  const upstreamBindings = await validateUpstreamBindings({
    samples,
    cases,
    catalogHash,
    apiOraclePath: path.resolve(options.apiOracle),
    fixtureReceiptPath: path.resolve(options.fixtureReceipt),
    receiptPaths: options.edgeReceipts,
    receipts,
  });
  if (!isObject(options.adminStates)) {
    throw new InputError("four admin auth-state paths are required");
  }
  const adminStates = {};
  for (const combination of COMBINATIONS) {
    if (!nonempty(options.adminStates[combination])) {
      throw new InputError(`admin auth state is missing for ${combination}`);
    }
    adminStates[combination] = await assertSecretPath(
      path.resolve(options.adminStates[combination]),
      secretRoot,
      `admin auth state ${combination}`,
    );
  }
  const deniedState = await assertSecretPath(
    path.resolve(options.deniedState),
    secretRoot,
    "Clone B denied auth state",
  );
  const authBinding = await validateAuthAttestation({
    attestationPath: path.resolve(options.authAttestation),
    runId: options.runId,
    secretRoot,
    adminStates,
    deniedState,
  });
  const outputRelativeToSecrets = path.relative(secretRoot, outputPath);
  const artifactRelativeToSecrets = path.relative(secretRoot, artifactRoot);
  if (
    (!outputRelativeToSecrets.startsWith("..") && !path.isAbsolute(outputRelativeToSecrets)) ||
    (!artifactRelativeToSecrets.startsWith("..") &&
      !path.isAbsolute(artifactRelativeToSecrets))
  ) {
    throw new InputError("evidence output and artifact root must be outside secret root");
  }
  if (
    !nonempty(options.executorId) ||
    !nonempty(options.reviewerId) ||
    options.executorId === options.reviewerId
  ) {
    throw new InputError("independent --executor-id and --reviewer-id are required");
  }
  return {
    schema_version: SCHEMA_VERSION,
    gate: GATE,
    mode: "execute",
    run_id: options.runId,
    scenario_catalog_sha256: catalogHash,
    samples_sha256: await fileSha256(samplesPath),
    samples_manifest_sha256: samples.manifest_sha256,
    upstream_bindings: upstreamBindings,
    auth_attestation_sha256: authBinding.attestationSha256,
    edge_receipts: receiptEdges,
    case_count: cases.length,
    cases,
    secret_paths: { adminStates, deniedState },
    secret_values: authBinding.sensitiveValues,
    auth_runtime_expectations: authBinding.runtimeExpectations,
    auth_runtime_tokens: authBinding.runtimeTokens,
    artifact_root: artifactRoot,
    output_path: outputPath,
    executor_id: options.executorId,
    reviewer_id: options.reviewerId,
  };
}

function publicPlan(plan) {
  return {
    schema_version: SCHEMA_VERSION,
    gate: GATE,
    status: "PASS",
    mode: "dry-run",
    run_id: plan.run_id,
    scenario_catalog_sha256: plan.scenario_catalog_sha256,
    samples_sha256: plan.samples_sha256,
    samples_manifest_sha256: plan.samples_manifest_sha256,
    upstream_bindings: plan.upstream_bindings,
    auth_attestation_sha256: plan.auth_attestation_sha256,
    sealed_edges: plan.edge_receipts,
    case_count: plan.case_count,
    contexts: {
      admin: [...COMBINATIONS],
      denied: ["devplus_devplus"],
      viewports: Object.keys(VIEWPORTS),
    },
    auth_state_files_validated: 5,
    auth_state_paths_disclosed: false,
    cases: plan.cases.map(({ key, coverage }) => ({
      case_key: key,
      task_id: coverage.task_id,
      resource_ids: coverage.resource_ids,
      revision_ids: coverage.revision_ids,
      http_probe_count: coverage.http_probes.length,
    })),
  };
}

function unwrap(value) {
  if (isObject(value) && Object.hasOwn(value, "data")) return value.data;
  return value;
}

function sanitizeText(value, sensitiveValues = []) {
  let text = String(value).replace(
    SENSITIVE_TEXT_RE,
    "[redacted-sensitive-value]",
  );
  for (const secret of sensitiveValues) {
    if (secret) text = text.split(secret).join("[redacted-auth-state-value]");
  }
  return text;
}

function sanitizeUrl(raw, expectedOrigin) {
  try {
    const parsed = new URL(raw);
    if (parsed.origin !== expectedOrigin) return null;
    return `${parsed.origin}${parsed.pathname}`;
  } catch {
    return null;
  }
}

function guardAttemptKey(attempt, expectedOrigin) {
  try {
    const parsed = new URL(attempt.url);
    const expected = new URL(expectedOrigin);
    const sameAuthority =
      parsed.hostname === expected.hostname && parsed.port === expected.port;
    const protocolMatches =
      (attempt.method === "WEBSOCKET" &&
        parsed.protocol === (expected.protocol === "https:" ? "wss:" : "ws:")) ||
      (attempt.method !== "WEBSOCKET" && parsed.protocol === expected.protocol);
    if (
      !sameAuthority ||
      !protocolMatches ||
      parsed.username ||
      parsed.password ||
      parsed.search ||
      parsed.hash
    ) {
      return null;
    }
    return `${attempt.method} ${parsed.pathname}`;
  } catch {
    return null;
  }
}

export function classifyGuardAttempts(attempts, expectedOrigin) {
  const expected = [];
  const forbidden = [];
  const counts = new Map();
  for (const attempt of attempts) {
    const key = guardAttemptKey(attempt, expectedOrigin);
    const limit = key ? EXPECTED_GUARD_ATTEMPT_LIMITS.get(key) : undefined;
    if (limit === undefined) {
      forbidden.push(attempt);
      continue;
    }
    const count = (counts.get(key) || 0) + 1;
    counts.set(key, count);
    if (count > limit) {
      forbidden.push({ ...attempt, classification: "guard_count_exceeded" });
      continue;
    }
    expected.push({ ...attempt, classification: "expected_guard_observation" });
  }
  return { expected, forbidden };
}

function guardConsoleEntry(entry) {
  return (
    entry.level === "error" &&
    String(entry.text || "").includes("ERR_BLOCKED_BY_CLIENT")
  );
}

export function classifyGuardConsoleEntries(
  entries,
  expectedAttempts,
  expectedOrigin,
) {
  const annotated = entries.map((entry) => ({
    ...entry,
    expected_guard_observation: false,
  }));
  const remaining = expectedAttempts
    .map((attempt) => ({ attempt }))
    .filter(({ attempt }) => attempt.method === "POST");

  // URL-bearing errors must consume the exact matching confirmed attempt.
  for (const entry of annotated) {
    if (!guardConsoleEntry(entry) || !nonempty(entry.url)) continue;
    const entryKey = guardAttemptKey(
      { method: "POST", url: entry.url },
      expectedOrigin,
    );
    if (!entryKey) continue;
    const matchIndex = remaining.findIndex(
      ({ attempt }) => guardAttemptKey(attempt, expectedOrigin) === entryKey,
    );
    if (matchIndex < 0) continue;
    const [{ attempt }] = remaining.splice(matchIndex, 1);
    entry.expected_guard_observation = true;
    entry.expected_guard_attempt = guardAttemptKey(attempt, expectedOrigin);
  }

  // Playwright sometimes omits location for Inspector-generated blocked fetch
  // errors. Those may consume only one still-unmatched confirmed POST attempt.
  for (const entry of annotated) {
    if (
      !guardConsoleEntry(entry) ||
      nonempty(entry.url) ||
      entry.expected_guard_observation ||
      remaining.length === 0
    ) {
      continue;
    }
    const [{ attempt }] = remaining.splice(0, 1);
    entry.expected_guard_observation = true;
    entry.expected_guard_attempt = guardAttemptKey(attempt, expectedOrigin);
  }
  return annotated;
}

export function classifyCompatibilityConsoleEntries(
  entries,
  network,
  resourceOracleKind,
  expectedOrigin,
  taskId,
) {
  const approval =
    resourceOracleKind === "legacy_frontend_task_snapshot"
      ? {
          paths: new Set([
            `/v1/tasks/${taskId}/predictions`,
            `/v1/tasks/${taskId}/audit-supplements`,
          ]),
          status: 404,
        }
      : resourceOracleKind === "frontend_rollback_compatibility"
        ? {
            paths: new Set([`/v1/tasks/${taskId}/resource-bundle`]),
            status: 404,
          }
        : resourceOracleKind === "v8_missing_resource_group"
          ? {
              paths: new Set([`/v1/tasks/${taskId}/resource-bundle`]),
              status: 409,
            }
          : null;
  return entries.map((entry) => {
    const annotated = {
      ...entry,
      expected_compatibility_observation: false,
    };
    if (
      !approval ||
      entry.level !== "error" ||
      !String(entry.text || "").includes(`status of ${approval.status}`) ||
      !nonempty(entry.url)
    ) {
      return annotated;
    }
    let parsed;
    try {
      parsed = new URL(entry.url);
    } catch {
      return annotated;
    }
    if (
      parsed.origin !== expectedOrigin ||
      !approval.paths.has(parsed.pathname)
    ) {
      return annotated;
    }
    const confirmed = network.some((request) => {
      try {
        const requestUrl = new URL(request.url);
        return (
          request.method === "GET" &&
          request.status === approval.status &&
          requestUrl.origin === expectedOrigin &&
          requestUrl.pathname === parsed.pathname
        );
      } catch {
        return false;
      }
    });
    if (confirmed) {
      annotated.expected_compatibility_observation = true;
      annotated.expected_compatibility_route = parsed.pathname;
      annotated.expected_compatibility_status = approval.status;
    }
    return annotated;
  });
}

function scrubObject(value, sensitiveValues = []) {
  if (Array.isArray(value)) {
    return value.map((child) => scrubObject(child, sensitiveValues));
  }
  if (!isObject(value)) {
    if (typeof value !== "string") return value;
    const trimmed = value.trim();
    if (
      (trimmed.startsWith("{") && trimmed.endsWith("}")) ||
      (trimmed.startsWith("[") && trimmed.endsWith("]"))
    ) {
      try {
        return JSON.stringify(
          scrubObject(JSON.parse(value), sensitiveValues),
        );
      } catch {
        // Fall through to conservative text redaction.
      }
    }
    return sanitizeText(value, sensitiveValues);
  }
  const result = {};
  for (const [key, child] of Object.entries(value)) {
    if (SENSITIVE_KEY_RE.test(key)) continue;
    result[key] = scrubObject(child, sensitiveValues);
  }
  return result;
}

async function importWorkspaceModule(moduleName) {
  const here = path.dirname(fileURLToPath(import.meta.url));
  const repoRoot = path.resolve(here, "../..");
  const packageRoot = path.join(repoRoot, "vue", "node_modules", moduleName);
  const packageDocument = JSON.parse(
    await fs.readFile(path.join(packageRoot, "package.json"), "utf8"),
  );
  const entry = packageDocument.module || packageDocument.main || "index.js";
  let entryPath = path.join(packageRoot, entry);
  try {
    await fs.access(entryPath);
  } catch {
    entryPath = `${entryPath}.js`;
  }
  const imported = await import(pathToFileURL(entryPath).href);
  return imported.default || imported;
}

async function sanitizeTrace(tracePath, sensitiveValues) {
  const JSZip = await importWorkspaceModule("jszip");
  const zip = await JSZip.loadAsync(await fs.readFile(tracePath));
  for (const name of Object.keys(zip.files)) {
    if (name === "resources" || name.startsWith("resources/")) zip.remove(name);
  }
  for (const name of ["trace.trace", "trace.network"]) {
    const entry = zip.file(name);
    if (!entry) throw new InputError(`Playwright trace is missing ${name}`);
    const source = await entry.async("string");
    const sanitized = source
      .split(/\r?\n/)
      .filter((line) => line.trim() !== "")
      .map((line) => {
        try {
          return JSON.stringify(scrubObject(JSON.parse(line), sensitiveValues));
        } catch {
          throw new InputError(`Playwright ${name} contains invalid JSONL`);
        }
      })
      .join("\n");
    zip.file(name, `${sanitized}\n`);
  }
  for (const [name, entry] of Object.entries(zip.files)) {
    if (entry.dir) continue;
    const payload = Buffer.from(await entry.async("uint8array"));
    for (const secret of sensitiveValues) {
      if (secret && payload.includes(Buffer.from(secret))) {
        throw new InputError(`sanitized Playwright trace still contains auth data in ${name}`);
      }
    }
  }
  await fs.writeFile(
    tracePath,
    await zip.generateAsync({ type: "nodebuffer", compression: "DEFLATE" }),
  );
}

async function stopAndSanitizeTrace(
  browserContext,
  tracePath,
  sensitiveValues,
  traceSanitizer,
) {
  try {
    await browserContext.tracing.stop({ path: tracePath });
    await traceSanitizer(tracePath, sensitiveValues);
  } catch (error) {
    await fs.rm(tracePath, { force: true }).catch(() => {});
    const detail = error instanceof Error ? error.message : String(error);
    throw new Error(`Playwright trace sanitization failed: ${detail}`);
  }
}

function collectResponse(cache, response, origin) {
  const safeUrl = sanitizeUrl(response.url(), origin);
  if (!safeUrl) return;
  const request = response.request();
  const method = request.method().toUpperCase();
  cache.network.push({ method, url: safeUrl, status: response.status() });
  if (MUTATING_METHODS.has(method)) {
    cache.mutatingRequests.push(`${method} ${safeUrl}`);
  }
  const contentType = response.headers()["content-type"] || "";
  if (method === "GET" && contentType.includes("json")) {
    const pending = response
      .body()
      .then((bodyBytes) =>
        cache.json.push({
          path: new URL(response.url()).pathname,
          body: JSON.parse(bodyBytes.toString("utf8")),
          body_sha256: crypto.createHash("sha256").update(bodyBytes).digest("hex"),
        }),
      )
      .catch(() => {});
    cache.pending.push(pending);
  }
}

async function installReadOnlyGuard(context, label) {
  const attempts = [];
  await context.exposeBinding("__g7ReportServiceWorkerAttempt", (_, scriptUrl) => {
    attempts.push({
      method: "SERVICE_WORKER",
      url: sanitizeText(scriptUrl),
      boundary: label,
    });
  });
  await context.addInitScript(() => {
    if (!navigator.serviceWorker?.register) return;
    navigator.serviceWorker.register = (...args) => {
      void window.__g7ReportServiceWorkerAttempt(String(args[0] ?? ""));
      return Promise.reject(
        new DOMException(
          "Service worker registration blocked by G7 read-only guard",
          "SecurityError",
        ),
      );
    };
  });
  context.on("serviceworker", (worker) => {
    attempts.push({
      method: "SERVICE_WORKER",
      url: sanitizeText(worker.url()),
      boundary: label,
    });
  });
  await context.route("**/*", async (route) => {
    const request = route.request();
    const method = request.method().toUpperCase();
    if (!READ_ONLY_METHODS.has(method)) {
      attempts.push({
        method,
        url: sanitizeText(request.url()),
        boundary: label,
      });
      await route.abort("blockedbyclient");
      return;
    }
    await route.fallback();
  });
  if (typeof context.routeWebSocket === "function") {
    await context.routeWebSocket("**/*", async (webSocket) => {
      attempts.push({
        method: "WEBSOCKET",
        url: sanitizeText(webSocket.url()),
        boundary: label,
      });
      await webSocket.close({ code: 1008, reason: "G7 read-only evidence run" });
    });
  } else {
    throw new InputError(
      "installed Playwright does not support fail-closed WebSocket routing",
    );
  }
  return attempts;
}

function newestJson(cache, exactPath) {
  for (let index = cache.json.length - 1; index >= 0; index -= 1) {
    if (cache.json[index].path === exactPath) return unwrap(cache.json[index].body);
  }
  return null;
}

function newestJsonRecord(cache, exactPath) {
  for (let index = cache.json.length - 1; index >= 0; index -= 1) {
    if (cache.json[index].path === exactPath) return cache.json[index];
  }
  return null;
}

function taskIdFromPayload(value) {
  const payload = unwrap(value);
  if (!isObject(payload)) return null;
  for (const candidate of [payload.id, payload.task_id, payload.task?.id]) {
    if (positiveInt(candidate)) return candidate;
  }
  return null;
}

function taskPayload(cache, taskId) {
  const exact = newestJson(cache, `/v1/tasks/${taskId}`);
  if (isObject(exact)) return exact;
  const detail = newestJson(cache, `/v1/tasks/${taskId}/detail`);
  if (isObject(detail?.task)) return detail.task;
  return isObject(detail) ? detail : null;
}

function bundlePayload(cache, taskId) {
  const bundle = newestJson(cache, `/v1/tasks/${taskId}/resource-bundle`);
  return isObject(bundle) ? bundle : null;
}

function groupScopeRefId(group) {
  if (nonnegativeInt(group.scope_ref_id)) return group.scope_ref_id;
  if (group.scope_kind === "task") return 0;
  if (group.scope_kind === "sku") return group.task_sku_item_id;
  if (group.scope_kind === "retouch_requirement") {
    return group.retouch_requirement_id;
  }
  return null;
}

export function groupMatchesLocator(group, locator, taskId) {
  if (!isObject(group) || !nonempty(locator)) return false;
  const runtimeMatch = RUNTIME_GROUP_LOCATOR_RE.exec(locator);
  if (runtimeMatch) {
    return positiveInt(group.id) && group.id === Number(runtimeMatch[1]);
  }
  const canonicalMatch = CANONICAL_GROUP_LOCATOR_RE.exec(locator);
  if (!canonicalMatch) return false;
  const ownerTask = group.task_id ?? taskId;
  return (
    positiveInt(ownerTask) &&
    ownerTask === Number(canonicalMatch[1]) &&
    nonempty(group.scope_kind) &&
    group.scope_kind === canonicalMatch[2] &&
    nonnegativeInt(groupScopeRefId(group)) &&
    groupScopeRefId(group) === Number(canonicalMatch[3])
  );
}

function matchingGroupIndexes(groups, resourceIds, taskId) {
  return groups
    .map((group, index) => ({
      index,
      matches: resourceIds.some((locator) =>
        groupMatchesLocator(group, locator, taskId),
      ),
    }))
    .filter((row) => row.matches)
    .map((row) => row.index);
}

function observedResourceLocators(groups, resourceIds, taskId) {
  return new Set(
    resourceIds.filter((locator) =>
      groups.some((group) => groupMatchesLocator(group, locator, taskId)),
    ),
  );
}

function groupsFromBundle(bundle) {
  return Array.isArray(bundle?.groups) ? bundle.groups : [];
}

function observedActions(cache, taskId, checkpoint, hook) {
  if (isObject(hook?.allowed_actions) && Array.isArray(hook.allowed_actions[checkpoint])) {
    return hook.allowed_actions[checkpoint].map(String).sort();
  }
  if (checkpoint === "task_detail") {
    const task = taskPayload(cache, taskId);
    if (Array.isArray(task?.allowed_actions)) return task.allowed_actions.map(String).sort();
  }
  if (checkpoint === "resource_bundle") {
    const bundle = bundlePayload(cache, taskId);
    if (Array.isArray(bundle?.allowed_actions)) {
      return bundle.allowed_actions.map(String).sort();
    }
  }
  return null;
}

function revisionBodies(cache) {
  return cache.json
    .filter((entry) => /^\/v1\/resource-groups\/\d+\/revisions$/.test(entry.path))
    .map((entry) => unwrap(entry.body))
    .filter(isObject);
}

function revisionIdsFromBodies(cache) {
  const ids = [];
  for (const body of revisionBodies(cache)) {
    for (const row of Array.isArray(body.items) ? body.items : []) {
      if (positiveInt(row?.id)) ids.push(row.id);
    }
  }
  return [...new Set(ids)];
}

async function openHistoryDrawers(page, groupIndexes) {
  const openResources = page.locator(".resource-rail header button").first();
  if ((await openResources.count()) === 0) {
    return { opened: false, uiComplete: false };
  }
  await openResources.click();
  const historyButtons = page.locator(".revision-history-button");
  await historyButtons.first().waitFor({ state: "visible", timeout: 10_000 });
  const available = await historyButtons.count();
  const indexes = groupIndexes.length
    ? groupIndexes
    : Array.from({ length: available }, (_, index) => index);
  if (indexes.some((index) => index < 0 || index >= available)) {
    return { opened: false, uiComplete: false };
  }
  let uiComplete = true;
  for (const [position, index] of indexes.entries()) {
    await historyButtons.nth(index).click();
    const drawer = page.locator(".revision-drawer");
    await drawer.waitFor({ state: "visible", timeout: 10_000 });
    await drawer
      .locator(".drawer-state")
      .filter({ hasText: /正在|loading/i })
      .waitFor({ state: "hidden", timeout: 10_000 })
      .catch(() => {});
    let pages = 0;
    while (pages < 100) {
      const cards = drawer.locator(".revision-card");
      const cardCount = await cards.count();
      const metaCount = await drawer.locator(".revision-card .revision-meta").count();
      const fileCount = await drawer.locator(".revision-card .revision-files").count();
      uiComplete &&= cardCount > 0 && metaCount === cardCount && fileCount >= cardCount;
      const next = drawer.locator('[aria-label="下一页历史修订"]').first();
      if ((await next.count()) === 0 || (await next.isDisabled())) break;
      await next.click();
      await page.waitForLoadState("networkidle", { timeout: 10_000 }).catch(() => {});
      pages += 1;
    }
    if (position < indexes.length - 1) {
      await drawer.locator('[aria-label="关闭历史修订"]').click();
      await drawer.waitFor({ state: "hidden", timeout: 10_000 });
    }
  }
  return { opened: true, uiComplete };
}

async function visibleSnapshot(page) {
  return page.evaluate(() => {
    const isVisible = (element) => {
      const style = window.getComputedStyle(element);
      return (
        !element.hidden &&
        element.getAttribute("aria-hidden") !== "true" &&
        style.display !== "none" &&
        style.visibility !== "hidden" &&
        style.opacity !== "0" &&
        element.getClientRects().length > 0
      );
    };
    const hook =
      typeof window.__G7_EVIDENCE__ === "object" && window.__G7_EVIDENCE__ !== null
        ? window.__G7_EVIDENCE__
        : null;
    const text = document.body?.innerText || "";
    const buttons = [...document.querySelectorAll("button")]
      .filter(isVisible)
      .map((element) => ({
        text: element.textContent?.trim() || "",
        aria: element.getAttribute("aria-label") || "",
        disabled: element.disabled,
      }));
    const actionControls = [
      ...document.querySelectorAll(
        [
          "button",
          '[role="button"]',
          'input[type="button"]',
          'input[type="submit"]',
          "[data-action]",
          "[data-action-key]",
          "[data-action-id]",
        ].join(","),
      ),
    ]
      .filter(isVisible)
      .map((element) => ({
        tag: element.tagName.toLowerCase(),
        text:
          element.textContent?.trim() ||
          (typeof element.value === "string" ? element.value.trim() : ""),
        aria: element.getAttribute("aria-label") || "",
        title: element.getAttribute("title") || "",
        action: element.getAttribute("data-action") || "",
        action_key: element.getAttribute("data-action-key") || "",
        action_id: element.getAttribute("data-action-id") || "",
        name: element.getAttribute("name") || "",
        value: element.getAttribute("value") || "",
      }));
    return {
      title: document.title,
      text: text.slice(0, 200_000),
      buttons,
      actionControls,
      actionControlSnapshotComplete: true,
      hook,
      taskPageVisible: Boolean(document.querySelector(".task-detail-view")),
      historyDrawerVisible: Boolean(document.querySelector(".revision-drawer")),
      revisionCardCount: document.querySelectorAll(".revision-card").length,
      revisionMetaCount: document.querySelectorAll(".revision-card .revision-meta").length,
      revisionFileSectionCount: document.querySelectorAll(
        ".revision-card .revision-files",
      ).length,
    };
  });
}

export function retiredActionsAbsent(observed, allowedActions) {
  if (
    observed?.actionControlSnapshotComplete !== true ||
    !Array.isArray(observed.actionControls) ||
    !Array.isArray(allowedActions)
  ) {
    return false;
  }
  const apiActionsAbsent = !allowedActions.some((action) =>
    RETIRED_ACTION_RE.test(String(action)),
  );
  const domActionsAbsent = !observed.actionControls.some((control) =>
    RETIRED_ACTION_RE.test(
      [
        control?.text,
        control?.aria,
        control?.title,
        control?.action,
        control?.action_key,
        control?.action_id,
        control?.name,
        control?.value,
      ]
        .map((value) => String(value || ""))
        .join(" "),
    ),
  );
  return apiActionsAbsent && domActionsAbsent;
}

function assertionByScenario(name, observed, coverage, cache) {
  const task = taskPayload(cache, coverage.task_id);
  const bundle = bundlePayload(cache, coverage.task_id);
  const groups = groupsFromBundle(bundle);
  const allActions = Array.isArray(task?.allowed_actions)
    ? task.allowed_actions.map(String)
    : [];
  if (name === "retired_actions_absent") {
    return retiredActionsAbsent(observed, allActions);
  }
  const hookValue = observed.hook?.assertions?.[name];
  if (typeof hookValue === "boolean") return hookValue;
  if (name === "terminal_actions_absent") {
    return !allActions.some((action) => /create|submit|approve|reject|upload|assign|takeover/i.test(action));
  }
  if (name === "no_cross_scope_assets") {
    return groups.every(
      (group) =>
        !positiveInt(group.task_id) ||
        group.task_id === coverage.task_id,
    );
  }
  if (name === "planning_fields_match") {
    return (
      task?.task_type === "sku_planning" &&
      (isObject(task?.planning) ||
        Array.isArray(task?.sku_items) ||
        Array.isArray(task?.planning_skus))
    );
  }
  if (name === "permission_denied_ui") {
    return coverage.http_probes.some((probe) => probe.expected_status === 403);
  }
  if (name === "historical_unavailable_ui") {
    const histories = revisionBodies(cache);
    return (
      coverage.http_probes.some((probe) => probe.expected_status === 410) &&
      (observed.text.includes("不可用") ||
        histories.some((body) =>
          canonicalJson(body).includes("historical_unavailable"),
        ))
    );
  }
  if (name === "negative_state_rendered") {
    return Boolean(
      observed.hook?.negative_state ||
        observed.text.match(/缺失|异常|不可用|未迁移|错误|missing|invalid/i),
    );
  }
  if (name === "approved_compatibility_difference_only") {
    return observed.taskPageVisible && cache.mutatingRequests.length === 0;
  }
  return false;
}

function safeArchivePath(value) {
  if (
    !nonempty(value) ||
    value.startsWith("/") ||
    value.startsWith("\\") ||
    value.includes("\\") ||
    value.split("/").includes("..")
  ) {
    return false;
  }
  return !value.split("/").some((segment) => segment === "");
}

export function validateSourceBundleManifest(
  manifest,
  expected,
  archiveEntryBytes,
) {
  if (
    !isObject(manifest) ||
    manifest.version !== 1 ||
    manifest.deterministic_profile !== "zip-stored-fixed-1980-0644-v1" ||
    !Array.isArray(manifest.members) ||
    !isObject(expected) ||
    !Array.isArray(expected.ordered_member_task_asset_ids) ||
    !isObject(archiveEntryBytes)
  ) {
    return false;
  }
  const members = manifest.members;
  if (
    members.length < 2 ||
    canonicalJson(members.map((member) => member?.task_asset_id)) !==
      canonicalJson(expected.ordered_member_task_asset_ids)
  ) {
    return false;
  }
  const archivePaths = members.map((member) => member?.archive_path);
  if (
    new Set(archivePaths).size !== archivePaths.length ||
    archivePaths.some((archivePath) => !safeArchivePath(archivePath))
  ) {
    return false;
  }
  const expectedEntries = ["manifest.json", ...archivePaths].sort();
  const actualEntries = Object.keys(archiveEntryBytes).sort();
  if (canonicalJson(actualEntries) !== canonicalJson(expectedEntries)) {
    return false;
  }
  return members.every((member) => {
    const bytes = archiveEntryBytes[member.archive_path];
    return (
      member.confirmed === true &&
      SHA256_RE.test(String(member.sha256 || "")) &&
      Buffer.isBuffer(bytes) &&
      bytesSha256(bytes) === member.sha256
    );
  });
}

function expectedSourceBundles(sample) {
  if (!Array.isArray(sample?.revision_facts)) return [];
  return sample.revision_facts
    .filter((fact) => isObject(fact?.source_bundle))
    .map((fact) => ({
      resource_key: fact.resource_key,
      predicted_revision_id: fact.predicted_revision_id,
      revision_no: fact.revision_no,
      task_asset_id: fact.source_bundle.task_asset_id,
      bundle_sha256: fact.source_bundle.bundle_sha256,
      ordered_member_task_asset_ids:
        fact.source_bundle.ordered_member_task_asset_ids,
    }));
}

function sourceBundleRevision(group, expected) {
  return [group?.working_revision, group?.finalized_revision]
    .filter(isObject)
    .find(
      (revision) =>
        revision?.source_file?.task_asset_id === expected.task_asset_id &&
        (!positiveInt(expected.predicted_revision_id) ||
          revision.id === expected.predicted_revision_id) &&
        (!positiveInt(expected.revision_no) ||
          revision.revision_no === expected.revision_no),
    );
}

async function sourceBundleArchiveEntries(bytes) {
  const JSZip = await importWorkspaceModule("jszip");
  const zip = await JSZip.loadAsync(bytes, {
    checkCRC32: true,
    createFolders: false,
  });
  const fileNames = Object.keys(zip.files).filter(
    (fileName) => !zip.files[fileName].dir,
  );
  const entries = {};
  for (const fileName of fileNames) {
    entries[fileName] = await zip.files[fileName].async("nodebuffer");
  }
  return entries;
}

async function verifySourceBundles({
  browserContext,
  origin,
  token,
  sample,
  coverage,
  bundle,
  network,
  verificationCache,
}) {
  const expectedBundles = expectedSourceBundles(sample);
  if (expectedBundles.length === 0 || !nonempty(token)) return false;
  const groups = groupsFromBundle(bundle);
  for (const expected of expectedBundles) {
    if (
      !nonempty(expected.resource_key) ||
      !positiveInt(expected.task_asset_id) ||
      !SHA256_RE.test(String(expected.bundle_sha256 || "")) ||
      !Array.isArray(expected.ordered_member_task_asset_ids) ||
      expected.ordered_member_task_asset_ids.length < 2 ||
      expected.ordered_member_task_asset_ids.some(
        (taskAssetId) => !positiveInt(taskAssetId),
      )
    ) {
      return false;
    }
    const group = groups.find((candidate) =>
      groupMatchesLocator(candidate, expected.resource_key, coverage.task_id),
    );
    const revision = sourceBundleRevision(group, expected);
    const sourceFile = revision?.source_file;
    if (
      !isObject(sourceFile) ||
      sourceFile.task_asset_id !== expected.task_asset_id ||
      sourceFile.mime_type !== "application/zip" ||
      !String(sourceFile.file_name || "").toLowerCase().endsWith(".zip") ||
      !positiveInt(sourceFile.file_size)
    ) {
      return false;
    }
    const cacheKey = [
      origin,
      expected.task_asset_id,
      expected.bundle_sha256,
      canonicalJson(expected.ordered_member_task_asset_ids),
    ].join("|");
    let verification = verificationCache.get(cacheKey);
    if (!verification) {
      verification = (async () => {
        const headers = { Authorization: `Bearer ${token}` };
        const metadataPath = `/v1/task-assets/${expected.task_asset_id}/download`;
        const metadataUrl = `${origin}${metadataPath}`;
        const metadataResponse = await browserContext.request.get(metadataUrl, {
          headers,
          maxRedirects: 0,
        });
        network.push({
          method: "GET",
          url: metadataUrl,
          status: metadataResponse.status(),
        });
        if (metadataResponse.status() !== 200) return false;
        let metadata;
        try {
          metadata = await metadataResponse.json();
        } catch {
          return false;
        }
        const download = metadata?.data;
        if (
          !isObject(download) ||
          download.download_mode !== "proxy" ||
          !nonempty(download.download_url) ||
          download.filename !== sourceFile.file_name ||
          download.mime_type !== sourceFile.mime_type ||
          download.file_size !== sourceFile.file_size
        ) {
          return false;
        }
        let downloadPath;
        try {
          downloadPath = safeRelativeUrl(
            download.download_url,
            "source bundle download_url",
          );
        } catch {
          return false;
        }
        const downloadUrl = `${origin}${downloadPath}`;
        const archiveResponse = await browserContext.request.get(downloadUrl, {
          headers,
          maxRedirects: 0,
        });
        network.push({
          method: "GET",
          url: sanitizeUrl(downloadUrl, origin) || `${origin}/invalid-download-url`,
          status: archiveResponse.status(),
        });
        if (archiveResponse.status() !== 200) return false;
        const archiveBytes = await archiveResponse.body();
        if (
          archiveBytes.length !== sourceFile.file_size ||
          bytesSha256(archiveBytes) !== expected.bundle_sha256
        ) {
          return false;
        }
        const entries = await sourceBundleArchiveEntries(archiveBytes);
        const manifestBytes = entries["manifest.json"];
        if (!Buffer.isBuffer(manifestBytes)) return false;
        let manifest;
        try {
          manifest = JSON.parse(manifestBytes.toString("utf8"));
        } catch {
          return false;
        }
        return validateSourceBundleManifest(manifest, expected, entries);
      })();
      verificationCache.set(cacheKey, verification);
    }
    if (!(await verification)) return false;
  }
  return true;
}

async function inspectDirectResponse(response, expectedUrl, label, requireJson = false) {
  if (!response) throw new Error(`${label} returned no response`);
  if (
    response.url() !== expectedUrl ||
    response.request().redirectedFrom() !== null
  ) {
    throw new Error(`${label} redirected away from the attested endpoint`);
  }
  const contentType = String(response.headers()["content-type"] || "").toLowerCase();
  const body = await response.body();
  const prefix = body.subarray(0, 512).toString("utf8").trim().toLowerCase();
  if (
    !contentType ||
    contentType.includes("text/html") ||
    prefix.startsWith("<!doctype html") ||
    prefix.startsWith("<html") ||
    body.length === 0
  ) {
    throw new Error(`${label} returned empty or HTML/login content`);
  }
  if (!requireJson) return { body, contentType };
  if (!contentType.includes("json")) {
    throw new Error(`${label} did not return JSON`);
  }
  try {
    return { body: JSON.parse(body.toString("utf8")), contentType };
  } catch {
    throw new Error(`${label} returned invalid JSON`);
  }
}

function runtimeIdentity(value) {
  const unwrapped = unwrap(value);
  const profile = isObject(unwrapped?.user) ? unwrapped.user : unwrapped;
  if (!isObject(profile)) return null;
  const identityId = profile.id ?? profile.user_id;
  const roles = Array.isArray(profile.roles)
    ? profile.roles.map(String)
    : nonempty(profile.role)
      ? [profile.role]
      : [];
  if (
    (typeof identityId !== "string" && !positiveInt(identityId)) ||
    roles.length === 0
  ) {
    return null;
  }
  return { identityId: String(identityId), roles };
}

async function inspectDirectApiResponse(response, expectedUrl, label) {
  if (response.url() !== expectedUrl) {
    throw new Error(`${label} redirected away from the attested endpoint`);
  }
  const contentType = String(response.headers()["content-type"] || "").toLowerCase();
  const body = await response.body();
  const prefix = body.subarray(0, 512).toString("utf8").trim().toLowerCase();
  if (
    !contentType.includes("json") ||
    prefix.startsWith("<!doctype html") ||
    prefix.startsWith("<html") ||
    body.length === 0
  ) {
    throw new Error(`${label} returned empty, non-JSON, or HTML/login content`);
  }
  try {
    return JSON.parse(body.toString("utf8"));
  } catch {
    throw new Error(`${label} returned invalid JSON`);
  }
}

async function verifyRuntimeIdentity(
  context,
  expectation,
  accessToken,
  label,
  identityRequester,
) {
  const expectedUrl = `${expectation.origin}/v1/auth/me`;
  let response;
  try {
    response = await identityRequester(context, expectedUrl, {
      headers: {
        accept: "application/json",
        authorization: `Bearer ${accessToken}`,
      },
      failOnStatusCode: false,
      maxRedirects: 0,
      timeout: 15_000,
    });
    if (response.status() !== 200) {
      throw new Error(`${label} /v1/auth/me did not return HTTP 200`);
    }
    const inspected = await inspectDirectApiResponse(
      response,
      expectedUrl,
      `${label} /v1/auth/me`,
    );
    const actual = runtimeIdentity(inspected);
    if (
      !actual ||
      actual.identityId !== String(expectation.identity_id) ||
      !actual.roles.includes(expectation.role)
    ) {
      throw new Error(
        `${label} /v1/auth/me identity or role differs from attestation`,
      );
    }
  } finally {
    await response?.dispose();
  }
}

async function probeStatuses(
  coverage,
  adminContext,
  deniedContext,
  origin,
  network,
  adminToken,
  deniedToken,
  probeRequester,
) {
  const rows = [];
  for (const probe of coverage.http_probes) {
    const probeUrl = `${origin}${probe.path}`;
    const requestOptions = (token) => ({
      headers: {
        accept: "application/json",
        authorization: `Bearer ${token}`,
      },
      failOnStatusCode: false,
      maxRedirects: 0,
      timeout: 15_000,
    });
    if (probe.expected_status === 403) {
      let adminResponse;
      try {
        adminResponse = await probeRequester(
          adminContext,
          probeUrl,
          requestOptions(adminToken),
        );
        if (
          !adminResponse ||
          adminResponse.status() < 200 ||
          adminResponse.status() >= 300
        ) {
          throw new Error(
            `HTTP probe ${probe.kind} admin positive control did not return 2xx`,
          );
        }
        await inspectDirectApiResponse(
          adminResponse,
          probeUrl,
          `HTTP probe ${probe.kind} admin positive control`,
        );
        network.push({
          method: "GET",
          url: probeUrl,
          status: adminResponse.status(),
        });
      } finally {
        await adminResponse?.dispose();
      }
    }
    const context =
      probe.expected_status === 403 ? deniedContext : adminContext;
    const token = probe.expected_status === 403 ? deniedToken : adminToken;
    let response;
    try {
      response = await probeRequester(
        context,
        probeUrl,
        requestOptions(token),
      );
      if (!response) {
        throw new Error(`HTTP probe ${probe.kind} returned no response`);
      }
      await inspectDirectApiResponse(
        response,
        probeUrl,
        `HTTP probe ${probe.kind}`,
      );
      network.push({
        method: "GET",
        url: probeUrl,
        status: response.status(),
      });
      rows.push({
        name: probe.kind,
        expected_status: probe.expected_status,
        actual_status: response.status(),
      });
    } finally {
      await response?.dispose();
    }
  }
  return rows;
}

async function writeJsonExclusive(filePath, value) {
  await fs.mkdir(path.dirname(filePath), { recursive: true });
  const handle = await fs.open(filePath, "wx", 0o600);
  try {
    await handle.writeFile(`${JSON.stringify(value, null, 2)}\n`, "utf8");
  } finally {
    await handle.close();
  }
}

function safeCaseDirectory(key) {
  return key.replaceAll("/", "__").replace(/[^A-Za-z0-9_.-]/g, "_");
}

async function artifactDescriptor(kind, filePath, artifactRoot) {
  return {
    kind,
    path: path.relative(artifactRoot, filePath).split(path.sep).join("/"),
    sha256: await fileSha256(filePath),
  };
}

async function executeCase({
  browserContext,
  deniedContext,
  item,
  plan,
  viewport,
  adminGuardAttempts,
  deniedGuardAttempts,
  traceSanitizer,
  bundleVerifier,
  bundleVerificationCache,
  probeRequester,
}) {
  const { scenario, sample, coverage, key } = item;
  const origin = ORIGINS[coverage.combination];
  const caseDir = path.join(plan.artifact_root, "playwright", safeCaseDirectory(key));
  await fs.mkdir(caseDir, { recursive: false });
  const screenshotPath = path.join(caseDir, "screenshot.png");
  const consolePath = path.join(caseDir, "console.json");
  const networkPath = path.join(caseDir, "network.json");
  const tracePath = path.join(caseDir, "trace.zip");
  const cache = { json: [], network: [], mutatingRequests: [], pending: [] };
  const consoleEntries = [];
  const adminGuardStart = adminGuardAttempts.length;
  const deniedGuardStart = deniedGuardAttempts.length;
  const startedEpochMs = Date.now();
  const startedMonotonicMs = performance.now();
  const startedAt = new Date(startedEpochMs).toISOString();
  await browserContext.tracing.start({
    screenshots: true,
    snapshots: true,
    sources: false,
  });
  const page = await browserContext.newPage();
  const deniedPage = await deniedContext.newPage();
  page.on("console", (message) => {
    const levelMap = { debug: "log", verbose: "log", warning: "warning", warn: "warning" };
    const level = levelMap[message.type()] || message.type();
    if (["log", "info", "warning", "error"].includes(level)) {
      consoleEntries.push({
        level,
        text: sanitizeText(message.text()),
        url: sanitizeText(message.location()?.url || ""),
      });
    }
  });
  page.on("response", (response) => collectResponse(cache, response, origin));
  let recordStatus = "PASS";
  let failureDetail = "";
  let traceStopAttempted = false;
  try {
    const navigationPath = positiveInt(coverage.task_id)
      ? `/tasks/${coverage.task_id}?g7_scenario=${encodeURIComponent(scenario.id)}`
      : `/?g7_scenario=${encodeURIComponent(scenario.id)}`;
    await page.goto(`${origin}${navigationPath}`, {
      waitUntil: "domcontentloaded",
      timeout: 30_000,
    });
    await page.waitForLoadState("networkidle", { timeout: 15_000 }).catch(() => {});
    await page.locator("body").waitFor({ state: "visible", timeout: 10_000 });
    await Promise.allSettled(cache.pending);
    const initialBundle = bundlePayload(cache, coverage.task_id);
    const initialGroups = groupsFromBundle(initialBundle);
    const selectedGroupIndexes = matchingGroupIndexes(
      initialGroups,
      coverage.resource_ids,
      coverage.task_id,
    );
    const selectedGroupIds = selectedGroupIndexes
      .map((index) => initialGroups[index]?.id)
      .filter(positiveInt);
    let historyUi = { opened: false, uiComplete: false };
    if (coverage.requirements.requires_history_drawer) {
      historyUi = await openHistoryDrawers(page, selectedGroupIndexes);
      if (!historyUi.opened) {
        throw new Error("history drawer trigger was not available");
      }
      await page.waitForLoadState("networkidle", { timeout: 10_000 }).catch(() => {});
      await Promise.allSettled(cache.pending);
    } else if (coverage.requirements.requires_revision_ids) {
      for (const groupId of selectedGroupIds) {
        const status = await page.evaluate(async (url) => {
          const response = await fetch(url, { credentials: "include" });
          await response.json();
          return response.status;
        }, `/v1/resource-groups/${groupId}/revisions?page=1&page_size=50`);
        if (status !== 200) {
          throw new Error(`revision history GET returned HTTP ${status}`);
        }
      }
      await Promise.allSettled(cache.pending);
    }
    const observed = await visibleSnapshot(page);
    const task = taskPayload(cache, coverage.task_id);
    const bundle = bundlePayload(cache, coverage.task_id);
    const observedLocators = observedResourceLocators(
      groupsFromBundle(bundle),
      coverage.resource_ids,
      coverage.task_id,
    );
    const observedRevisionIds = revisionIdsFromBodies(cache);
    const allowedActions = coverage.allowed_actions.map((row) => {
      const observedActionsValue = observedActions(
        cache,
        coverage.task_id,
        row.checkpoint,
        observed.hook,
      );
      return {
        checkpoint: row.checkpoint,
        expected: row.expected,
        observed: observedActionsValue || [],
      };
    });
    const httpStatuses = await probeStatuses(
      coverage,
      browserContext,
      deniedContext,
      origin,
      cache.network,
      plan.auth_runtime_tokens.admin[coverage.combination],
      plan.auth_runtime_tokens.denied,
      probeRequester,
    );
    const pageMatches =
      observed.taskPageVisible &&
      taskIdFromPayload(task) === coverage.task_id &&
      new URL(page.url()).origin === origin;
    const resourceOracle = coverage.resource_oracle;
    const taskResponse = newestJsonRecord(
      cache,
      `/v1/tasks/${coverage.task_id}`,
    );
    const assetsMatch =
      resourceOracle.kind === "v8_resource_groups"
        ? coverage.resource_ids.every((resourceId) =>
            observedLocators.has(resourceId),
          ) &&
          (!coverage.requirements.requires_revision_ids ||
            canonicalJson([...coverage.revision_ids].sort((a, b) => a - b)) ===
              canonicalJson([...observedRevisionIds].sort((a, b) => a - b)))
        : [
              "v8_missing_resource_group",
              "v8_expected_no_resource_groups",
            ].includes(resourceOracle.kind)
          ? groupsFromBundle(bundle).length === 0 &&
            coverage.resource_ids.length === 0 &&
            coverage.revision_ids.length === 0
          : taskResponse?.body_sha256 === resourceOracle.task_response_sha256;
    const assertions = {
      page_matches_manifest: pageMatches,
      assets_match: assetsMatch,
      allowed_actions: allowedActions,
      history_drawer: coverage.requirements.requires_history_drawer
        ? {
            opened: historyUi.opened && observed.historyDrawerVisible,
            stage_status_actor_file_time_checked: historyUi.uiComplete,
            revision_ids: coverage.revision_ids,
          }
        : null,
      http_statuses: httpStatuses,
      oracle_sha256: coverage.oracle_sha256,
      resource_oracle_kind: resourceOracle.kind,
    };
    const bundleMembersVerified =
      coverage.requirements.required_assertions.includes(
        "bundle_members_match",
      )
        ? await bundleVerifier({
            browserContext,
            origin,
            token: plan.auth_runtime_tokens.admin[coverage.combination],
            sample,
            coverage,
            bundle,
            network: cache.network,
            verificationCache: bundleVerificationCache,
          })
        : null;
    for (const assertionName of coverage.requirements.required_assertions) {
      if (assertionName === "allowed_actions_exact") continue;
      if (assertionName === "page_matches_manifest") {
        assertions[assertionName] = pageMatches;
      } else if (assertionName === "assets_match") {
        assertions[assertionName] = assetsMatch;
      } else if (assertionName === "bundle_members_match") {
        assertions[assertionName] = bundleMembersVerified;
      } else {
        assertions[assertionName] = assertionByScenario(
          assertionName,
          observed,
          coverage,
          cache,
        );
      }
    }
    const blockedAttempts = [
      ...adminGuardAttempts.slice(adminGuardStart),
      ...deniedGuardAttempts.slice(deniedGuardStart),
    ];
    const guardClassification = classifyGuardAttempts(blockedAttempts, origin);
    const classifiedConsoleEntries = classifyCompatibilityConsoleEntries(
      classifyGuardConsoleEntries(
        consoleEntries,
        guardClassification.expected,
        origin,
      ),
      cache.network,
      resourceOracle.kind,
      origin,
      coverage.task_id,
    );
    const unexpectedConsoleErrors = classifiedConsoleEntries.filter(
      (entry) =>
        entry.level === "error" &&
        !entry.expected_guard_observation &&
        !entry.expected_compatibility_observation,
    ).length;
    const fiveXxCount = cache.network.filter(
      (request) => request.status >= 500,
    ).length;
    assertions.console_unexpected_error_count = unexpectedConsoleErrors;
    assertions.network_5xx_count = fiveXxCount;
    assertions.read_only_guard = {
      expected_guard_observation_count: guardClassification.expected.length,
      forbidden_attempt_count: guardClassification.forbidden.length,
      mutation_response_count: cache.mutatingRequests.length,
    };
    const failedAssertions = coverage.requirements.required_assertions.filter(
      (name) => {
        if (name === "allowed_actions_exact") {
          return allowedActions.some(
            (row) => canonicalJson(row.expected) !== canonicalJson(row.observed),
          );
        }
        return assertions[name] !== true;
      },
    );
    const failedStatuses = httpStatuses.filter(
      (row) => row.expected_status !== row.actual_status,
    );
    if (
      failedAssertions.length ||
      failedStatuses.length ||
      unexpectedConsoleErrors ||
      fiveXxCount ||
      cache.mutatingRequests.length ||
      guardClassification.forbidden.length
    ) {
      throw new Error(
        [
          failedAssertions.length ? `assertions=${failedAssertions.join(",")}` : "",
          failedStatuses.length ? "HTTP probe mismatch" : "",
          unexpectedConsoleErrors ? `console_errors=${unexpectedConsoleErrors}` : "",
          fiveXxCount ? `network_5xx=${fiveXxCount}` : "",
          cache.mutatingRequests.length
            ? `mutating_requests=${cache.mutatingRequests.join(",")}`
            : "",
          guardClassification.forbidden.length
            ? `blocked_forbidden_requests=${guardClassification.forbidden
                .map((row) => `${row.method} ${row.url}`)
                .join(",")}`
            : "",
        ]
          .filter(Boolean)
          .join("; "),
      );
    }
    await page.screenshot({ path: screenshotPath, fullPage: false });
    const consoleArtifact = {
      schema_version: SCHEMA_VERSION,
      case_key: key,
      source_kind: SOURCE_KIND,
      unexpected_error_count: unexpectedConsoleErrors,
      entries: classifiedConsoleEntries,
    };
    const networkArtifact = {
      schema_version: SCHEMA_VERSION,
      case_key: key,
      source_kind: SOURCE_KIND,
      five_xx_count: fiveXxCount,
      requests: cache.network,
    };
    await writeJsonExclusive(consolePath, consoleArtifact);
    await writeJsonExclusive(networkPath, networkArtifact);
    traceStopAttempted = true;
    await stopAndSanitizeTrace(
      browserContext,
      tracePath,
      plan.secret_values,
      traceSanitizer,
    );
    const artifacts = await Promise.all([
      artifactDescriptor("screenshot", screenshotPath, plan.artifact_root),
      artifactDescriptor("console", consolePath, plan.artifact_root),
      artifactDescriptor("network", networkPath, plan.artifact_root),
      artifactDescriptor("trace", tracePath, plan.artifact_root),
    ]);
    const record = {
      schema_version: SCHEMA_VERSION,
      source_kind: SOURCE_KIND,
      scenario_id: scenario.id,
      combination: coverage.combination,
      viewport: coverage.viewport,
      status: "PASS",
      executor_id: plan.executor_id,
      reviewer_id: plan.reviewer_id,
      started_at: startedAt,
      finished_at: finishedAtFromMonotonic(
        startedEpochMs,
        startedMonotonicMs,
      ),
      url: page.url(),
      samples_sha256: plan.samples_sha256,
      sample_sha256: sample.sample_sha256,
      edge_identity: {
        edge: plan.edge_receipts[coverage.combination].edge,
        frontend_sha256:
          plan.edge_receipts[coverage.combination].frontend_sha256,
        backend_sha256: plan.edge_receipts[coverage.combination].backend_sha256,
        fixture_identity:
          plan.edge_receipts[coverage.combination].fixture_identity,
      },
      viewport_spec: VIEWPORTS[viewport],
      task_id: coverage.task_id,
      revision_ids: coverage.revision_ids,
      resource_ids: coverage.resource_ids,
      assertions,
      artifacts,
    };
    record.record_sha256 = canonicalSha256(record);
    return record;
  } catch (error) {
    recordStatus = "FAIL";
    failureDetail = sanitizeText(error instanceof Error ? error.message : String(error));
    if (!traceStopAttempted) {
      traceStopAttempted = true;
      try {
        await stopAndSanitizeTrace(
          browserContext,
          tracePath,
          plan.secret_values,
          traceSanitizer,
        );
      } catch (traceError) {
        const traceDetail = sanitizeText(
          traceError instanceof Error ? traceError.message : String(traceError),
        );
        failureDetail = `${failureDetail}; ${traceDetail}`;
      }
    }
    throw new Error(`${key}: ${failureDetail}`);
  } finally {
    await Promise.allSettled([page.close(), deniedPage.close()]);
    if (recordStatus === "FAIL") {
      await writeJsonExclusive(path.join(caseDir, "failure.json"), {
        schema_version: SCHEMA_VERSION,
        case_key: key,
        status: recordStatus,
        detail: failureDetail,
      }).catch(() => {});
    }
  }
}

export async function executePlan(plan, testHooks = {}) {
  const traceSanitizer = testHooks.traceSanitizer || sanitizeTrace;
  const bundleVerifier = testHooks.bundleVerifier || verifySourceBundles;
  const probeRequester =
    testHooks.probeRequester ||
    ((context, url, requestOptions) =>
      context.request.get(url, requestOptions));
  const identityRequester =
    testHooks.identityRequester ||
    ((context, url, requestOptions) =>
      context.request.get(url, requestOptions));
  if (typeof traceSanitizer !== "function") {
    throw new InputError("trace sanitizer must be callable");
  }
  if (typeof identityRequester !== "function") {
    throw new InputError("identity requester must be callable");
  }
  if (typeof bundleVerifier !== "function") {
    throw new InputError("source bundle verifier must be callable");
  }
  if (typeof probeRequester !== "function") {
    throw new InputError("HTTP probe requester must be callable");
  }
  await fs.mkdir(plan.artifact_root, { recursive: true });
  await fs.mkdir(path.join(plan.artifact_root, "playwright"), { recursive: false });
  try {
    await fs.access(plan.output_path);
    throw new InputError("playwright evidence output already exists");
  } catch (error) {
    if (error instanceof InputError) throw error;
    if (error.code !== "ENOENT") throw error;
  }
  const playwright = await importWorkspaceModule("playwright");
  const browser = await playwright.chromium.launch({ headless: true });
  const records = [];
  const bundleVerificationCache = new Map();
  try {
    for (const viewport of Object.keys(VIEWPORTS)) {
      const viewportSpec = VIEWPORTS[viewport];
      const contexts = {};
      const guardAttempts = {};
      for (const combination of COMBINATIONS) {
        contexts[combination] = await browser.newContext({
          storageState: plan.secret_paths.adminStates[combination],
          viewport: { width: viewportSpec.width, height: viewportSpec.height },
          deviceScaleFactor: viewportSpec.device_scale_factor,
          serviceWorkers: "block",
        });
        if (typeof testHooks.installContextRoutes === "function") {
          await testHooks.installContextRoutes(contexts[combination], {
            combination,
            role: "admin",
          });
        }
        guardAttempts[combination] = await installReadOnlyGuard(
          contexts[combination],
          `${combination}/${viewport}/admin`,
        );
        await verifyRuntimeIdentity(
          contexts[combination],
          plan.auth_runtime_expectations.admin[combination],
          plan.auth_runtime_tokens.admin[combination],
          `${combination}/${viewport}/admin`,
          identityRequester,
        );
      }
      const deniedContext = await browser.newContext({
        storageState: plan.secret_paths.deniedState,
        viewport: { width: viewportSpec.width, height: viewportSpec.height },
        deviceScaleFactor: viewportSpec.device_scale_factor,
        serviceWorkers: "block",
      });
      if (typeof testHooks.installContextRoutes === "function") {
        await testHooks.installContextRoutes(deniedContext, {
          combination: "devplus_devplus",
          role: "denied",
        });
      }
      const deniedGuardAttempts = await installReadOnlyGuard(
        deniedContext,
        `devplus_devplus/${viewport}/denied`,
      );
      await verifyRuntimeIdentity(
        deniedContext,
        plan.auth_runtime_expectations.denied,
        plan.auth_runtime_tokens.denied,
        `devplus_devplus/${viewport}/denied`,
        identityRequester,
      );
      try {
        for (const item of plan.cases.filter(
          ({ coverage }) => coverage.viewport === viewport,
        )) {
          records.push(
            await executeCase({
              browserContext: contexts[item.coverage.combination],
              deniedContext,
              item,
              plan,
              viewport,
              adminGuardAttempts: guardAttempts[item.coverage.combination],
              deniedGuardAttempts,
              traceSanitizer,
              bundleVerifier,
              bundleVerificationCache,
              probeRequester,
            }),
          );
        }
      } finally {
        await Promise.allSettled([
          ...Object.values(contexts).map((context) => context.close()),
          deniedContext.close(),
        ]);
      }
    }
  } finally {
    await browser.close();
  }
  if (records.length !== plan.case_count) {
    throw new Error(
      `execution produced ${records.length} records for ${plan.case_count} planned cases`,
    );
  }
  const evidence = {
    schema_version: SCHEMA_VERSION,
    source_kind: SOURCE_KIND,
    scenario_catalog_sha256: plan.scenario_catalog_sha256,
    samples_sha256: plan.samples_sha256,
    run_id: plan.run_id,
    records,
  };
  await writeJsonExclusive(plan.output_path, evidence);
  return evidence;
}

function parseAssignments(values, label) {
  const result = {};
  for (const value of values) {
    const separator = value.indexOf("=");
    if (separator <= 0) throw new InputError(`${label} must use combination=path`);
    const combination = value.slice(0, separator);
    const filePath = value.slice(separator + 1);
    if (!COMBINATIONS.includes(combination) || result[combination] || !nonempty(filePath)) {
      throw new InputError(`${label} contains an invalid or duplicate combination`);
    }
    result[combination] = filePath;
  }
  return result;
}

export function parseArgs(argv) {
  const values = new Map();
  const repeated = new Map([
    ["--edge-receipt", []],
    ["--admin-state", []],
  ]);
  let mode = null;
  for (let index = 0; index < argv.length; index += 1) {
    const arg = argv[index];
    if (arg === "--dry-run" || arg === "--execute") {
      if (mode) throw new InputError("choose exactly one of --dry-run or --execute");
      mode = arg.slice(2);
      continue;
    }
    if (!arg.startsWith("--")) throw new InputError(`unexpected argument ${arg}`);
    const value = argv[index + 1];
    if (value === undefined || value.startsWith("--")) {
      throw new InputError(`${arg} requires a value`);
    }
    index += 1;
    if (repeated.has(arg)) repeated.get(arg).push(value);
    else if (values.has(arg)) throw new InputError(`${arg} may only be specified once`);
    else values.set(arg, value);
  }
  if (!mode) throw new InputError("choose --dry-run or --execute");
  const required = [
    "--scenarios",
    "--samples",
    "--api-oracle",
    "--fixture-receipt",
    "--auth-attestation",
    "--secret-root",
    "--denied-state",
    "--artifact-root",
    "--output",
    "--run-id",
    "--executor-id",
    "--reviewer-id",
  ];
  for (const name of required) {
    if (!values.has(name)) throw new InputError(`${name} is required`);
  }
  return {
    mode,
    scenarios: values.get("--scenarios"),
    samples: values.get("--samples"),
    apiOracle: values.get("--api-oracle"),
    fixtureReceipt: values.get("--fixture-receipt"),
    authAttestation: values.get("--auth-attestation"),
    edgeReceipts: repeated.get("--edge-receipt"),
    secretRoot: values.get("--secret-root"),
    adminStates: parseAssignments(repeated.get("--admin-state"), "--admin-state"),
    deniedState: values.get("--denied-state"),
    artifactRoot: values.get("--artifact-root"),
    output: values.get("--output"),
    runId: values.get("--run-id"),
    executorId: values.get("--executor-id"),
    reviewerId: values.get("--reviewer-id"),
  };
}

async function main() {
  const options = parseArgs(process.argv.slice(2));
  const plan = await buildPlan(options);
  if (options.mode === "dry-run") {
    await writeJsonExclusive(plan.output_path, publicPlan(plan));
    process.stdout.write(
      `${JSON.stringify({ status: "PASS", mode: "dry-run", case_count: plan.case_count })}\n`,
    );
    return;
  }
  const evidence = await executePlan(plan);
  process.stdout.write(
    `${JSON.stringify({ status: "PASS", mode: "execute", record_count: evidence.records.length })}\n`,
  );
}

const isMain =
  process.argv[1] &&
  path.resolve(process.argv[1]) === path.resolve(fileURLToPath(import.meta.url));
if (isMain) {
  main().catch((error) => {
    process.stderr.write(
      `${JSON.stringify({
        status: "FAIL",
        error: sanitizeText(error instanceof Error ? error.message : String(error)),
      })}\n`,
    );
    process.exitCode = 1;
  });
}
