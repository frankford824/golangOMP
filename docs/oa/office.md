# OA 人事与财务 API

身份集成由入口在主站鉴权成功后调用 `office.ApplyIdentity(r.Context(), extensionDB, sourceReadOnly, actor)`。人员档案读主站用户和有效角色，扩展库只保存 OA 自有记录。

费用节点 `manager` 取人员 `manager_id`，指定人员写 `fixed:<用户ID>`；无可用直属负责人时使用环境变量 `OA_FALLBACK_APPROVER_ID`。费用文档 `route` 为账号 ID 数组，`current` 从 0 开始。模板管理可请求 `/templates?draft=1`，委托列表 `/delegates`。

人员动作 `people.manage`，财务动作 `finance.pay`、`finance.report`。超管设置模板委托 `draft` / `publish`。

开户、组织、角色、姓名、工号、启停由主站原生授权接口维护。入职、离职、返聘需要先完成主站账号操作，再登记当日 OA 任职元数据。Excel 确认仅写 OA 用工备注与店铺任职。超管可用 PUT `/oa-api/finance/actions/{id}` 写入 `{pay,report}`，付款、报表和模板发布均需独立授权。

费用收件箱使用 OA 扩展表，通过 GET `/oa-api/finance/notifications` 返回当前账号通知，负载为 `{document_id,state,site,title}`。

订单范围集成调用 `office.ShopOverrides(ctx, extensionDB, actorID)`，按店铺覆盖基础经营范围。原生业务角色以主站有效授予为准。

- GET `/oa-api/people/list` → `{items:[{id,name,employee_no,department_id,team_id,employment_type,joined,left,roles,management,version}]}`，可选 `q`。
- GET `/oa-api/people/export` 下载 xlsx。POST `/oa-api/people/preview` multipart `file` → `{token,items:[{row,name,error}]}`。POST `/oa-api/people/confirm` JSON `{token}` → `{count}`。
- GET `/oa-api/people/{id}` → 人员对象含 `terms`, `events`。POST `/oa-api/people/{id}/change` JSON `{kind,effective_date,department_id,team_id,shop_id,target_id,roles,management,employment_type,version}`。`kind` 为 `hire`, `transfer`, `shop_transfer`, `leave`, `handover`, `rehire`, `roles`。
- GET `/oa-api/finance/templates` → `{items:[{id,name,type,channels,fields,required,evidence,nodes,threshold_cents,extra,version}]}`。PUT `/oa-api/finance/templates/{id}` 提交模板字段并传 `publish`。PUT `/oa-api/finance/delegates/{id}` JSON `{level}`，`level` 为 `none` / `draft` / `publish`。
- GET `/oa-api/finance/documents?queue=mine|todo|pay` → `{items:[文档]}`。POST `/oa-api/finance/documents` JSON `{request_id,template_id,amount_cents,reason,channel,holder,account,bank,extra,files,site}`。`site` 为 `main` / `asset`，`files` 为上传返回的 ID 数组。
- POST `/oa-api/finance/files` multipart `file` → `{id,name}`。GET `/oa-api/finance/files/{id}` 下载。
- GET `/oa-api/finance/documents/{id}` → `{id,applicant_id,template,amount_cents,reason,channel,holder,account,bank,extra,files,site,state,route,current,history,payment}`。
- POST `/oa-api/finance/documents/{id}/resubmit` 使用同一申请输入结构。POST `/oa-api/finance/documents/{id}/decision` JSON `{decision,comment,current}`，决策为 `approve` / `return` / `reject`。POST `/oa-api/finance/documents/{id}/payment` JSON `{amount_cents,date,receipt}`。
- GET `/oa-api/finance/report?period=YYYY[-MM[-DD]]` → `{amount_cents,count,items:[文档]}`。
