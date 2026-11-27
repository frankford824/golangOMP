// @vitest-environment jsdom
import { mount, flushPromises, enableAutoUnmount } from "@vue/test-utils";
import { beforeEach, afterEach, describe, it, expect, vi } from "vitest";
enableAutoUnmount(afterEach);
const mocks = vi.hoisted(() => ({
  list: vi.fn(),
  counts: vi.fn(),
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
  reviewed: false,
  token: "visible-baseline",
  updated_at: "2026-10-23T00:00:00Z",
};
beforeEach(() => {
  window.scrollTo = vi.fn();
  vi.clearAllMocks();
  mocks.can.mockReturnValue(true);
  mocks.counts.mockReset().mockResolvedValue({ total: 230, pending: 205 });
  mocks.list.mockResolvedValue({
    data: [row],
    next_before_id: 0,
    refreshed_at: "2026-10-23T00:00:00Z",
  });
  mocks.maintain.mockResolvedValue({
    ...row,
    reviewed: true,
    status: "pending",
  });
  mocks.logs.mockResolvedValue({ data: [], pagination: { total: 0 } });
});
describe("completed SKU maintenance", () => {
  it("keeps the multi-code filter on the next page and restores keyword search for a single code", async () => {
    mocks.list.mockResolvedValue({ data: [row], next_before_id: 10, refreshed_at: row.updated_at });
    const wrapper = mount(CostMaintenanceList);
    await flushPromises();
    await wrapper.get("#cost-keyword").setValue("DZD000047 DZD000043，dzd000047");
    await wrapper.get(".maintenance-tools form").trigger("submit");
    await flushPromises();
    const filter = { sku_codes: "DZD000047,DZD000043", keyword: undefined, review_state: "pending" };
    expect(mocks.list.mock.lastCall?.[0]).toMatchObject({ ...filter, before_id: 0 });
    await wrapper.findAll("button").find(button => button.text() === "下一页")!.trigger("click");
    await flushPromises();
    expect(mocks.list.mock.lastCall?.[0]).toMatchObject({ ...filter, before_id: 10 });
    await wrapper.get("#cost-keyword").setValue("DZD000047");
    await wrapper.get(".maintenance-tools form").trigger("submit");
    await flushPromises();
    expect(mocks.list.mock.lastCall?.[0]).toMatchObject({ keyword: "DZD000047", sku_codes: undefined, before_id: 0 });
  });

  it("reports the filtered workload, removes a confirmed item and fills its place", async () => {
    const replacement = { ...row, id: 11, sku_code: "CGK002" };
    mocks.list.mockResolvedValueOnce({ data: [row], next_before_id: 10 })
      .mockResolvedValueOnce({ data: [replacement], next_before_id: 9 });
    const wrapper = mount(CostMaintenanceList);
    await flushPromises();
    expect(wrapper.get(".queue-summary").text()).toContain("待确认 205 条");
    expect(wrapper.get(".maintenance-footer").text()).toContain("共 3 页");
    await wrapper.findAll("button").find(button => button.text() === "确认")!.trigger("click");
    await flushPromises();
    expect(wrapper.get("tbody").text()).toContain("CGK002");
    expect(wrapper.get("tbody").text()).not.toContain("CGK001");
    expect(wrapper.get(".queue-summary").text()).toContain("待确认 204 条");
    expect(mocks.counts).toHaveBeenCalledTimes(1);
  });

  it("returns to the queue head if an older page has emptied", async () => {
    mocks.list.mockResolvedValueOnce({ data: [row], next_before_id: 10 })
      .mockResolvedValueOnce({ data: [], next_before_id: 0 })
      .mockResolvedValueOnce({ data: [row], next_before_id: 0 });
    const wrapper = mount(CostMaintenanceList);
    await flushPromises();
    await wrapper.findAll("button").find(button => button.text() === "下一页")!.trigger("click");
    await flushPromises();
    expect(mocks.list.mock.calls.map(call => call[0].before_id)).toEqual([0, 10, 0]);
    expect(wrapper.get("tbody").text()).toContain("CGK001");
    expect(wrapper.get(".maintenance-footer").text()).toContain("第 1 页");
  });

  it("can switch from all codes to pending and keeps the list usable if counts fail", async () => {
    mocks.counts.mockRejectedValue(new Error("timeout"));
    const wrapper = mount(CostMaintenanceList);
    await flushPromises();
    await wrapper.get("#cost-review-state").setValue("all");
    await wrapper.get(".queue-summary button").trigger("click");
    await flushPromises();
    expect(mocks.list.mock.lastCall?.[0]).toMatchObject({ review_state: "pending", page_size: 100, before_id: 0 });
    expect(wrapper.get(".queue-summary").text()).toContain("数量暂不可用");
    expect(wrapper.get("tbody").text()).toContain("CGK001");
  });

  it("defaults to pending and resets the cursor when switching to reviewed or all",async()=>{
    mocks.list.mockResolvedValue({data:[row],next_before_id:10,refreshed_at:row.updated_at});
    const w=mount(CostMaintenanceList);await flushPromises();
    expect(mocks.list.mock.calls[0][0]).toMatchObject({review_state:"pending",before_id:0});
    await w.findAll("button").find(b=>b.text()==="下一页")!.trigger("click");await flushPromises();
    await w.get("#cost-review-state").setValue("reviewed");await flushPromises();
    expect(mocks.list.mock.calls.at(-1)?.[0]).toMatchObject({review_state:"reviewed",before_id:0});
    await w.get("#cost-review-state").setValue("all");await flushPromises();
    expect(mocks.list.mock.calls.at(-1)?.[0]).toMatchObject({review_state:"all",before_id:0});
  });

  it.each(["pending", "synced"])(
    "replaces confirmation with a persistent reviewed label while %s and keeps editing available",
    async (status) => {
      mocks.list.mockResolvedValue({
        data: [{ ...row, reviewed: true, status }],
        next_before_id: 0,
        refreshed_at: row.updated_at,
      });
      const w = mount(CostMaintenanceList, {
        global: { stubs: { Teleport: true } },
      });
      await flushPromises();
      expect(w.findAll("button").some((b) => b.text() === "确认")).toBe(false);
      expect(w.get(".maintenance-reviewed").text()).toBe("已确认");
      const edit = w.findAll("button").find((b) => b.text() === "修改")!;
      expect(edit.attributes("disabled")).toBeUndefined();
      await edit.trigger("click");
      expect(w.get('form[role="dialog"]').text()).toContain("修改编码");
    },
  );
  it("still allows explicit conflict resolution after an earlier human review", async () => {
    mocks.list.mockResolvedValue({
      data: [{ ...row, reviewed: true, status: "conflict" }],
      next_before_id: 0,
      refreshed_at: row.updated_at,
    });
    const w = mount(CostMaintenanceList);
    await flushPromises();
    expect(
      w
        .findAll("button")
        .find((b) => b.text() === "确认")!
        .attributes("disabled"),
    ).toBeUndefined();
  });
  it("removes a committed review from pending even if the subsequent refresh fails", async () => {
    mocks.list
      .mockResolvedValueOnce({
        data: [row],
        next_before_id: 0,
        refreshed_at: row.updated_at,
      })
      .mockRejectedValueOnce(new Error("读取失败"));
    const w = mount(CostMaintenanceList);
    await flushPromises();
    await w
      .findAll("button")
      .find((b) => b.text() === "确认")!
      .trigger("click");
    await flushPromises();
    expect(w.find(".maintenance-reviewed").exists()).toBe(false);
    expect(w.findAll("button").some((b) => b.text() === "确认")).toBe(false);
    expect(mocks.maintain).toHaveBeenCalledTimes(1);
  });
  it("distinguishes metadata delivery from a reconciled price", async () => {
    mocks.list.mockResolvedValue({
      data: [
        {
          ...row,
          status: "synced",
          base_sync_required: true,
          base_sync_error: "ERP资料发送失败",
        },
      ],
      next_before_id: 0,
      refreshed_at: row.updated_at,
    });
    const w = mount(CostMaintenanceList, {
      global: { stubs: { Teleport: true } },
    });
    await flushPromises();
    expect(w.text()).toContain("ERP 已回读一致");
    expect(w.text()).toContain("资料同步失败");
    expect(w.findAll("button").some((b) => b.text() === "确认")).toBe(true);
  });
  it("confirms exactly the displayed system price baseline without editing product fields", async () => {
    mocks.list
      .mockResolvedValueOnce({
        data: [row],
        next_before_id: 0,
        refreshed_at: row.updated_at,
      })
      .mockResolvedValue({
        data: [],
        next_before_id: 0,
        refreshed_at: row.updated_at,
      });
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
    expect(w.find(".maintenance-reviewed").exists()).toBe(false);
    expect(w.findAll("button").some((b) => b.text() === "确认")).toBe(false);
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
  it("opens the replacement row when saving the last visible item and continuing", async () => {
    const replacement = { ...row, id: 11, sku_code: "CGK002" };
    mocks.list.mockResolvedValueOnce({ data: [row], next_before_id: 10 })
      .mockResolvedValueOnce({ data: [replacement], next_before_id: 9 });
    const wrapper = mount(CostMaintenanceList, { global: { stubs: { Teleport: true } } });
    await flushPromises();
    await wrapper.get("button.accent").trigger("click");
    const form = wrapper.get('form[role="dialog"]');
    await form.findAll('input[type="number"]')[3]!.setValue("4.4");
    await form.get('input[value="cost_incorrect"]').setValue(true);
    await form.trigger("submit", { submitter: form.get("button[data-next]").element });
    await flushPromises();
    expect(wrapper.get('form[role="dialog"]').text()).toContain("CGK002");
    expect(mocks.list).toHaveBeenCalledTimes(2);
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
