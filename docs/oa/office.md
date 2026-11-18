# OA 人事与财务 API

身份集成由入口在鉴权成功后调用 `office.ApplyIdentity(r.Context(), db, actor)` 并使用返回的 actor。返回 `false` 时拒绝全部 OA 访问。人员列表使用隔离 source `users(id,display_name,employee_no,department_id,team_id,status)`。

任职记录按生效日投影到 OA 请求身份。费用节点 `manager` 取人员 `manager_id`，指定人员写 `fixed:<用户ID>`；无可用直属负责人时使用环境变量 `OA_FALLBACK_APPROVER_ID`。财务授权名 `finance.pay`、`finance.report`，人员授权名 `people.manage`。费用文档 `route` 为账号 ID 数组，`current` 从 0 开始。模板管理可请求 `/templates?draft=1`，委托列表 `/delegates`。

人员动作 `people.manage`，财务动作 `finance.pay`、`finance.report`。超管设置模板委托 `draft` / `publish`。

新员工先通过 OA 沙箱 `/v1/users` 创建账号，再使用其稳定 ID 登记人员入职。Excel 预览返回姓名与工号差异，确认为同一事务更新沙箱账号与任职。空单元格保留原值，工号占用或预览后账号变化会阻止确认。超管可用 PUT `/oa-api/finance/actions/{id}` 写入 `{pay,report}`，两个字段均为布尔值。

网关启动后定时调用 `office.ApplyDueIdentityChanges`，登录及代理鉴权前执行同一函数。当天离职、返聘直接参与人员变更事务，未来变更从持久队列执行。两者仅连接 OA 沙箱数据库。费用收件箱使用 `notifications` 的 `oa_finance_document`，负载为 `{document_id,state,site,title}`。

订单范围集成可调用 `office.ShopTerms(ctx, db, actorID)`。第二个返回值表示已存在截至当日生效的本地任职事实；空店铺列表表示到期后没有本地店铺。调用方仍须校验该用户的经营权限并审查既有外部店铺范围。OA 人员角色补充 OA 请求身份，核心已有授权保持原值。

- GET `/oa-api/people/list` → `{items:[{id,name,employee_no,department_id,team_id,employment_type,joined,left,roles,management,version}]}`，可选 `q`。
- GET `/oa-api/people/export` 下载 xlsx。POST `/oa-api/people/preview` multipart `file` → `{token,items:[{row,name,error}]}`。POST `/oa-api/people/confirm` JSON `{token}` → `{count}`。
- GET `/oa-api/people/{id}` → 人员对象含 `terms`, `events`。POST `/oa-api/people/{id}/change` JSON `{kind,effective_date,department_id,team_id,shop_id,target_id,roles,management,employment_type,version}`。`kind` 为 `hire`, `transfer`, `shop_transfer`, `leave`, `handover`, `rehire`, `roles`。
- GET `/oa-api/finance/templates` → `{items:[{id,name,type,channels,fields,required,evidence,nodes,threshold_cents,extra,version}]}`。PUT `/oa-api/finance/templates/{id}` 提交模板字段并传 `publish`。PUT `/oa-api/finance/delegates/{id}` JSON `{level}`，`level` 为 `none` / `draft` / `publish`。
- GET `/oa-api/finance/documents?queue=mine|todo|pay` → `{items:[文档]}`。POST `/oa-api/finance/documents` JSON `{request_id,template_id,amount_cents,reason,channel,holder,account,bank,extra,files,site}`。`site` 为 `main` / `asset`，`files` 为上传返回的 ID 数组。
- POST `/oa-api/finance/files` multipart `file` → `{id,name}`。GET `/oa-api/finance/files/{id}` 下载。
- GET `/oa-api/finance/documents/{id}` → `{id,applicant_id,template,amount_cents,reason,channel,holder,account,bank,extra,files,site,state,route,current,history,payment}`。
- POST `/oa-api/finance/documents/{id}/resubmit` 使用同一申请输入结构。POST `/oa-api/finance/documents/{id}/decision` JSON `{decision,comment,current}`，决策为 `approve` / `return` / `reject`。POST `/oa-api/finance/documents/{id}/payment` JSON `{amount_cents,date,receipt}`。
- GET `/oa-api/finance/report?period=YYYY[-MM[-DD]]` → `{amount_cents,count,items:[文档]}`。
