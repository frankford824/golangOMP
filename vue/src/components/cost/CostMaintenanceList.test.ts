// @vitest-environment jsdom
import { mount, flushPromises, enableAutoUnmount } from "@vue/test-utils";
import { beforeEach, afterEach, describe, it, expect, vi } from "vitest";
enableAutoUnmount(afterEach);
const mocks = vi.hoisted(() => ({
  list: vi.fn(),
  maintain: vi.fn(),
  logs: vi.fn(),
  can: vi.fn((_permission: string) => true),
}));
vi.mock("@/composables/usePermission", () => ({
  usePermission: () => ({ can: mocks.can }),
}));
vi.mock("@/services/api/costMaintenanceApi", async (original) => ({
  ...(await original<typeof import("@/services/api/costMaintenanceApi")>()),
  costMaintenanceApi: mocks,
}));
import CostMaintenanceList from "./CostMaintenanceList.vue";
const row = {
  id: 10,
  task_id: 71,
  sku_code: "CGK001",
  product_name: "测试名称",
  style_code: "KT",
  width_cm: 20,
  height_cm: 30,
  area_m2: 0.06,
  size_text: "20×30cm",
  cost_input: null,
  cost_price: 3.3,
  erp_cost: 3.3,
  revision: 2,
  status: "baseline",
  token: "visible-baseline",
  updated_at: "2026-10-23T00:00:00Z",
};
beforeEach(() => {
  window.scrollTo = vi.fn();
  vi.clearAllMocks();
  mocks.can.mockReturnValue(true);
  mocks.list.mockResolvedValue({
    data: [row],
    next_before_id: 0,
    refreshed_at: "2026-10-23T00:00:00Z",
  });
  mocks.maintain.mockResolvedValue({ ...row, status: "pending" });
  mocks.logs.mockResolvedValue({ data: [], pagination: { total: 0 } });
});
describe("completed SKU maintenance", () => {
  it("confirms exactly the displayed system price baseline without editing product fields", async () => {
    const w = mount(CostMaintenanceList, {
      global: { stubs: { Teleport: true } },
    });
    await flushPromises();
    await w
      .findAll("button")
      .find((b) => b.text() === "确认")!
      .trigger("click");
    await flushPromises();
    expect(mocks.maintain).toHaveBeenCalledWith(10, {
      action: "confirm",
      token: "visible-baseline",
      revision: 2,
      erp_cost: 3.3,
    });
    expect(w.text()).toContain("系统已生效");
    expect(mocks.list).toHaveBeenCalledTimes(2);
  });
  it("requires a reason and submits only changed fields", async () => {
    const w = mount(CostMaintenanceList, {
      global: { stubs: { Teleport: true } },
    });
    await flushPromises();
    await w
      .findAll("button")
      .find((b) => b.text() === "修改")!
      .trigger("click");
    await flushPromises();
    const form = w.get('form[role="dialog"]');
    expect(
      form.get('button[type="submit"]').attributes("disabled"),
    ).toBeDefined();
    await form.findAll('input[type="number"]')[3]!.setValue("4.4");
    await form.get('input[value="cost_incorrect"]').setValue(true);
    await form.trigger("submit");
    await flushPromises();
    expect(mocks.maintain).toHaveBeenCalledWith(10, {
      action: "modify",
      token: "visible-baseline",
      revision: 2,
      erp_cost: 3.3,
      reason_code: "cost_incorrect",
      reason_detail: "",
      cost_price: 4.4,
    });
    expect(w.find('form[role="dialog"]').exists()).toBe(false);
  });
  it("keeps a stale edit visible and does not retry or silently replace its baseline", async () => {
    mocks.maintain.mockRejectedValue(
      Object.assign(new Error("记录已更新"), { status: 409 }),
    );
    const w = mount(CostMaintenanceList, {
      global: { stubs: { Teleport: true } },
    });
    await flushPromises();
    await w
      .findAll("button")
      .find((b) => b.text() === "修改")!
      .trigger("click");
    await flushPromises();
    const form = w.get('form[role="dialog"]');
    await form.findAll('input[type="number"]')[3]!.setValue("5.5");
    await form.get('input[value="cost_incorrect"]').setValue(true);
    await flushPromises();
    await form.trigger("submit");
    await flushPromises();
    expect(mocks.maintain).toHaveBeenCalledTimes(1);
    expect(w.get('form[role="dialog"]').text()).toContain("记录已更新");
    expect(
      w.get('form[role="dialog"] button[type="submit"]').attributes("disabled"),
    ).toBeDefined();
    expect(mocks.list).toHaveBeenCalledTimes(1);
  });
  it("paginates using the server cursor and resets when filters change", async () => {
    mocks.list.mockResolvedValue({
      data: [row],
      next_before_id: 10,
      refreshed_at: row.updated_at,
    });
    const w = mount(CostMaintenanceList, {
      global: { stubs: { Teleport: true } },
    });
    await flushPromises();
    await w
      .findAll("button")
      .find((b) => b.text() === "下一页")!
      .trigger("click");
    await flushPromises();
    expect(mocks.list.mock.calls.at(-1)?.[0].before_id).toBe(10);
    await w.get("#cost-status").setValue("conflict");
    await flushPromises();
    expect(mocks.list.mock.calls.at(-1)?.[0]).toMatchObject({
      before_id: 0,
      status: "conflict",
    });
  });
  it("disables mutation controls without both required capabilities", async () => {
    mocks.can.mockImplementation((p) => p !== "erp.manage");
    const w = mount(CostMaintenanceList, {
      global: { stubs: { Teleport: true } },
    });
    await flushPromises();
    expect(
      w
        .findAll("button")
        .find((b) => b.text() === "确认")!
        .attributes("disabled"),
    ).toBeDefined();
    expect(
      w
        .findAll("button")
        .find((b) => b.text() === "修改")!
        .attributes("disabled"),
    ).toBeDefined();
  });
  it("shows a log filter with structured reasons", async () => {
    const w = mount(CostMaintenanceList, {
      global: { stubs: { Teleport: true } },
    });
    await flushPromises();
    await w
      .findAll("button")
      .find((b) => b.text() === "修改日志")!
      .trigger("click");
    await flushPromises();
    await w.get('select[aria-label="日志原因"]').setValue("area_incorrect");
    await flushPromises();
    expect(mocks.logs).toHaveBeenLastCalledWith({
      keyword: "",
      reason_code: "area_incorrect",
      page: 1,
      page_size: 50,
    });
  });
});
