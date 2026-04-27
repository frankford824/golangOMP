#!/usr/bin/env bash
# Generates API usage and integration guides for each release.
# Called by package_release; writes into the staging docs directory.
set -euo pipefail

VERSION="${1:-v0.4}"
OUTPUT_DIR="${2:-}"
if [ -z "$OUTPUT_DIR" ]; then
  echo "Usage: $0 VERSION OUTPUT_DIR" >&2
  exit 1
fi
mkdir -p "$OUTPUT_DIR"
GEN_TIME="$(date -u +"%Y-%m-%d %H:%M:%S UTC")"

# ---------------------------------------------------------------------------
# API_USAGE_GUIDE.md
# ---------------------------------------------------------------------------
cat >"$OUTPUT_DIR/API_USAGE_GUIDE.md" <<'DOC'
# {VERSION} API 娴ｈ法鏁ょ拠瀛樻

## 1. 閻楀牊婀版穱鈩冧紖
- 閻楀牊婀伴崣? {VERSION}
- 閻㈢喐鍨氶弮鍫曟？: {GEN_TIME}
- 閺堝秴濮熺拠瀛樻: MAIN = 閸忣剙鍙℃稉姘鎼存梻鏁ら張宥呭 (缁旑垰褰?8080)閿涘瓓ridge = ERP/JST 闁倿鍘ら崳銊︽箛閸?(缁旑垰褰?8081)
- 闁劎璁查惄顔肩秿鐠囧瓨妲? 娴滄垶婀囬崝鈥虫珤 releases/{VERSION}/ 閹存牠鈧俺绻?current 鏉烆垶鎽奸幒銉問闂?

## 2. 閺堝秴濮熼崗銉ュ經
- MAIN base URL: `http://<host>:8080`閿涘牓绮拋銈忕礆
- Bridge base URL: `http://127.0.0.1:8081`閿涘牆鎮撻張娲劥缂冭绱?
- 闁村瓨娼堥弬鐟扮础: `Authorization: Bearer <token>` 閹存牕鍚嬬€?`X-Debug-Actor-Id` + `X-Debug-Actor-Roles`
- 鐠囬攱鐪版径? `Content-Type: application/json`閿涘矂澹岄弶鍐ㄦ倵閹碘偓閺堝澧犵粩顖濈熅閻㈤亶娓堕幖鍝勭敨閺堝鏅?token
- token/session: 閻ц缍嶉崥搴ょ箲閸?`token`閿涘苯鎮楃紒顓☆嚞濮瑰倸婀?Header 娑擃厽鎯＄敮锔肩幢`GET /v1/auth/me` 閸欘垶鐛欑拠浣哥秼閸撳秳绱扮拠?

## 3. 鐠併倛鐦夋稉搴ｆ暏閹?
- `GET /v1/auth/register-options` 閳?閼惧嘲褰囧▔銊ュ斀闁銆嶉敍鍫ュ劥闂?缂佸嫸绱?
- `POST /v1/auth/register` 閳?濞夈劌鍞介敍宀冾嚞濮瑰倸鐡у▓? username, password, display_name, department, team, mobile, email
- `POST /v1/auth/login` 閳?閻ц缍嶉敍宀冾嚞濮? username, password閿涙稑鎼锋惔鏂挎儓 user, token, frontend_access
- `GET /v1/auth/me` 閳?瑜版挸澧犻悽銊﹀煕閿涘矂娓?Bearer token閿涘矁绻戦崶?profile + frontend_access
- `PUT /v1/auth/password` 閳?娣囶喗鏁肩€靛棛鐖滈敍宀勬付 Bearer token
- `GET /v1/users` 閳?閻劍鍩涢崚妤勩€冮敍鍦歊/Admin 閺夊啴妾洪敍?
- `GET /v1/users/{id}` 閳?閻劍鍩涚拠锔藉剰
- `POST /v1/users/{id}/roles` 閳?閸掑棝鍘ょ憴鎺曞
- `DELETE /v1/users/{id}/roles/{role}` 閳?缁夊娅庣憴鎺曞
- 鐢瓕顫嗛柨娆掝嚖: 401 閺堫亣顓荤拠渚婄礉403 閺夊啴妾烘稉宥堝喕閿?00 閸欏倹鏆熼弽锟犵崣婢惰精瑙?

## 4. 缂佸嫮绮愭稉搴㈡綀闂?
- 闁劑妫?缂? 閸ュ搫鐣鹃弸姘閿涘矂鈧俺绻?register-options 閼惧嘲褰囬敍娑氭暏閹村嘲缍婄仦?department + team
- 鐡掑懐楠囩粻锛勬倞閸? 闁俺绻?config 闁板秶鐤嗛敍瀹╯_super_admin=true 閹枫儲婀侀崗銊╁劥閺夊啴妾?
- 闁劑妫粻锛勬倞閸? is_department_admin閿涘苯褰茬粻锛勬倞閺堫剟鍎撮梻銊ф暏閹磋渹绗岄幙宥勭稊鐠佹澘缍?
- frontend_access 缂佹挻鐎? menus, pages, actions, scopes, roles, is_super_admin, is_department_admin
- menus/pages/actions: 閸撳秶顏幑顔筋劃閹貉冨煑閼挎粌宕?妞ょ敻娼?閹垮秳缍旈惃鍕▔闂?

## 5. ERP 閺屻儴顕楅惄绋垮彠閹恒儱褰?
- `GET /v1/erp/products` 閳?娴溠冩惂閸掓銆冮敍灞炬暜閹?keyword, sku_code, category 缁涘鐡柅?
- `GET /v1/erp/products/{id}` 閳?娴溠冩惂鐠囷附鍎?
- `GET /v1/erp/categories` 閳?缁崵娲伴崚妤勩€?
- 閹碘偓閺堝娅ヨぐ鏇犳暏閹村嘲娼庨崣顖濐問闂傤喕绗傛潻?ERP 閺屻儴顕楅幒銉ュ經

## 6. 娴犺濮熸稉缁樼ウ缁嬪甯撮崣?
- `POST /v1/tasks` 閳?閸掓稑缂撴禒璇插
- `GET /v1/tasks` 閳?娴犺濮熼崚妤勩€冮敍鍫濆瀻妞ょ偣鈧胶鐡柅澶涚礆
- `GET /v1/tasks/board` 閳?娴犺濮熼惇瀣緲
- `GET /v1/tasks/{id}` 閳?娴犺濮熺拠锔藉剰
- `GET /v1/tasks/{id}/business-info` 閳?娑撴艾濮熸穱鈩冧紖閿涘湧ATCH 閸欘垱娲块弬甯礆
- `PATCH /v1/tasks/{id}/business-info` 閳?缂佸瓨濮㈡稉姘娣団剝浼?
- `GET /v1/tasks/{id}/detail` 閳?娴犺濮熺€瑰本鏆ｇ拠锔藉剰
- `POST /v1/tasks/{id}/assign` 閳?閸掑棝鍘?
- `POST /v1/tasks/{id}/submit-design` 閳?閹绘劒姘︾拋鎹愵吀
- `POST /v1/tasks/{id}/audit/claim` 閳?鐎光剝鐗虫０鍡楀絿
- `POST /v1/tasks/{id}/audit/approve` 閳?鐎光剝鐗抽柅姘崇箖
- `POST /v1/tasks/{id}/audit/reject` 閳?鐎光剝鐗虫す鍐叉礀
- `POST /v1/tasks/{id}/warehouse/receive` 閳?娴犳挸绨遍幒銉︽暪
- `POST /v1/tasks/{id}/warehouse/complete` 閳?娴犳挸绨辩€瑰本鍨?
- 閸忔湹绮?task/audit/warehouse/outsource/procurement 閻╃鍙ч幒銉ュ經鐟?OpenAPI

## 7. 閺冦儱绻旀稉搴☆吀鐠佲剝甯撮崣?
- `GET /v1/permission-logs` 閳?閺夊啴妾洪弮銉ョ箶閿涘湚R/Admin閿?
- `GET /v1/operation-logs` 閳?閹垮秳缍旂拋鏉跨秿閼辨艾鎮庨敍鍧盿sk events, export-job events, integration call logs閿?
- `GET /v1/integration/call-logs` 閳?闂嗗棙鍨氱拫鍐暏閺冦儱绻旈敍鍫ｅ閸撳秶顏棁鈧悽顭掔礆

## 8. 閸忔娊鏁弫鐗堝祦缂佹挻鐎?
- user: id, username, display_name, department, team, roles, frontend_access
- frontend_access: menus, pages, actions, scopes, is_super_admin, is_department_admin
- department/team: 閺嬫矮濡囬崐纭风礉鐟?config/frontend_access.json
- task summary: id, task_no, workflow, product_selection, procurement_summary
- operation log summary: actor, action, resource_type, resource_id, created_at

## 9. 鐢瓕顫嗛柨娆掝嚖閻椒绗岄懕鏃囩殶濞夈劍鍓版禍瀣€?
- 401 UNAUTHORIZED 閳?鐠併倛鐦夋径杈Е閿涘oken 閺冪姵鏅ラ幋鏍箖閺?
- 403 PERMISSION_DENIED 閳?閺夊啴妾烘稉宥堝喕
- 400 閸欏倹鏆熼弽锟犵崣婢惰精瑙?閳?濡偓閺屻儴顕Ч鍌欑秼鐎涙顔?
- ERP 閸愭瑨绔熼悾? 娴?business-info filed_at 缁涘妲戠涵顔兼簚閺咁垵袝閸?Bridge 閸愭瑥鍙?
- 閻喎鐤勯崘娆忓弳: 鐠嬨劍鍘ф担璺ㄦ暏 production 閻滎垰顣ㄩ敍灞界紦鐠侇喛浠堢拫鍐ㄥ帥閻?stub 閹存牗绁寸拠鏇炵氨
DOC

# Substitute version and time in API_USAGE_GUIDE
sed "s/{VERSION}/$VERSION/g; s/{GEN_TIME}/$GEN_TIME/g" "$OUTPUT_DIR/API_USAGE_GUIDE.md" > "$OUTPUT_DIR/API_USAGE_GUIDE.md.tmp"
mv "$OUTPUT_DIR/API_USAGE_GUIDE.md.tmp" "$OUTPUT_DIR/API_USAGE_GUIDE.md"

# ---------------------------------------------------------------------------
# API_INTEGRATION_GUIDE.md
# ---------------------------------------------------------------------------
cat >"$OUTPUT_DIR/API_INTEGRATION_GUIDE.md" <<'DOC'
# {VERSION} 閹恒儱褰涢懕鏃囩殶鐠囧瓨妲?

## 1. 閼辨棁鐨熼惄顔界垼
- 閺堫剛澧楅張顒勨偓鍌氭値閼辨棁鐨熼惃鍕瘜濞翠胶鈻? 濞夈劌鍞?閻ц缍嶉妴浣烘暏閹?鐟欐帟澹婇妴涓扲P 閺屻儴顕楅妴浣锋崲閸斺€冲灡瀵?閻婢?鐠囷附鍎忛妴浣割吀閺嶆悶鈧椒绮ㄦ惔鎾扁偓浣规綀闂勬劖妫╄箛妞尖偓浣规惙娴ｆ粏顔囪ぐ?
- 瑜版挸澧犳稉宥呯紦鐠侇喚娲块幒銉ュ竾濞?閻喎鐤勯崘娆忓弳閻ㄥ嫯绔熼悾? ERP Bridge 閸愭瑥鍙嗛妴浣烘晸娴溠冪氨閻╁瓨甯撮崘娆忓弳閵嗕焦婀崗鍛瀻妤犲矁鐦夐惃鍕闁插繑鎼锋担?

## 2. 閹恒劏宕橀懕鏃囩殶妞ゅ搫绨?
1. `GET /v1/auth/register-options` 閼惧嘲褰囧▔銊ュ斀闁銆?
2. `POST /v1/auth/register` 濞夈劌鍞介悽銊﹀煕
3. `POST /v1/auth/login` 閻ц缍?
4. `GET /v1/auth/me` 閼惧嘲褰囪ぐ鎾冲閻劍鍩?
5. 閺嶏繝鐛?`frontend_access` (menus, pages, actions)
6. 閹稿娼堥梽鎰潔缁€娲€夐棃顫礄ERP 閺屻儴顕楃€佃澧嶉張澶屾瑜版洜鏁ら幋宄板讲鐟欎緤绱?
7. `GET /v1/erp/products` 缁?ERP 閺屻儴顕?
8. 鐟欐帟澹婇崚鍡涘帳 `POST /v1/users/{id}/roles`閿涘矂鐛欑拠渚€銆夐棃銏犲綁閸?
9. 娴犺濮熸稉缁樼ウ缁? 閸掓稑缂?-> 閸掓銆?-> 閻婢?-> 鐠囷附鍎?-> 娑撴艾濮熸穱鈩冧紖 -> 鐎光剝鐗?-> 娴犳挸绨?
10. 閺冦儱绻旈弻銉ф箙: permission-logs, operation-logs閿涘牓娓?HR/Admin 閺夊啴妾洪敍?

## 3. 閻ц缍嶉幀浣峰▏閻劏顕╅弰?
- token 閺€鎯ф躬 Header: `Authorization: Bearer <token>`
- /me 閻劋绨懢宄板絿瑜版挸澧犻悽銊﹀煕閸?frontend_access閿涘本鐦″▎陇鐭鹃悽鍗炲瀼閹广垺鍨ㄩ崚閿嬫煀閸欘垵鐨熼悽銊︽降閺嶏繝鐛?
- 閺€鐟扮槕閸氬酣娓堕柌宥嗘煀閻ц缍嶉懢宄板絿閺?token

## 4. 閸撳秶顏い鐢告桨閺勯箖娈ｇ拠瀛樻
- menus: 閹貉冨煑娓氀嗙珶閺?妞ゅ爼鍎撮懣婊冨礋妞よ妯夐梾?
- pages: 閹貉冨煑鐠侯垳鏁?妞ょ敻娼扮拋鍧楁６閺夊啴妾?
- actions: 閹貉冨煑閹稿鎸?閹垮秳缍旈弶鍐
- 娑撳秴鎮撻柈銊╂，/鐟欐帟澹? 鐟?config/frontend_access.json閿涘畳efaults + department + team + roles + identities 閸氬牆鑻?
- 閹碘偓閺堝娅ヨぐ鏇犳暏閹村嘲褰茬憴? dashboard, erp_query, profile.me, profile.change_password
- 娴滃搫濮忕悰灞炬杺娑擃厼绺? 閸欘垵顫?user_admin, org_admin, role_admin, logs_center閿涘牆鎯?permission, operation, integration 閺冦儱绻旈敍?

## 5. 缂佸嫮绮愭稉搴ゅ閸欑柉浠堢拫鍐ㄧ紦鐠?
- 姒涙顓荤搾鍛獓缁狅紕鎮婇崨? 闁俺绻?AUTH_SUPER_ADMIN_USERNAMES 缁涘鍘ょ純顔煎灥婵瀵?
- 濞夈劌鍞介柈銊╂，閻劍鍩? register 閺冨爼鈧瀚?department + team
- 濞夈劌鍞介柈銊╂，缁狅紕鎮婇崨? 濞夈劌鍞介崥搴ㄢ偓姘崇箖 roles 閸掑棝鍘?Admin 閹?department_admin 閻╃鍙ч煬顐″敜
- 妤犲矁鐦夐柈銊╂，瀹割喖绱? 娑撳秴鎮?department 閻ц缍嶉崥?frontend_access.menus/pages 娑撳秴鎮?
- 鐟欐帟澹婇崣妯绘纯: 閸掑棝鍘?缁夊娅庣憴鎺曞閸氬酣鍣搁弬鎵瑜版洘鍨ㄧ拫?/me閿涘矂鐛欑拠?menus/pages/actions 閸欐ê瀵?

## 6. 娑撶粯绁︾粙瀣粓鐠嬪啫缂撶拋?
- 閻劍鍩?-> 閻ц缍?-> 娴犺濮?-> 閺冦儱绻? 閹稿甯归懡鎰粓鐠嬪啴銆庢惔蹇斿⒔鐞?
- 闁倸鎮庨惄瀛樺复閼辨棁鐨? 濞夈劌鍞介妴浣烘瑜版洏鈧?me閵嗕笒RP 閺屻儴顕楅妴浣锋崲閸斺€冲灙鐞?閻婢?鐠囷附鍎忛妴浣风瑹閸斺€蹭繆閹垬鈧礁顓搁弽鎼侇暙閸?闁俺绻?妞瑰啿娲栭妴浣风波鎼存挻甯撮弨?鐎瑰本鍨?
- 閻喎鐤勯崘娆忓弳鐠嬨劍鍘? business-info filed_at 娴兼俺袝閸?Bridge 娴溠冩惂閸愭瑥鍙嗛敍娑楁崲閸斺€冲灡瀵ゆ亽鈧礁顓搁弽鎼炩偓浣风波鎼存挾鐡戞导姘晸 DB

## 7. 鐢瓕顫嗛梻顕€顣介幒鎺撶叀
- 閻ц缍嶆径杈Е: 濡偓閺?username/password閿涘瞼鈥樼拋銈囨暏閹村嘲鍑″▔銊ュ斀娑撴梻濮搁幀?active
- /me 閺夊啴妾哄鍌氱埗: 濡偓閺?token 閺堝鏅ラ敍灞炬￥ 401
- frontend_access 娑撳秶顑侀崥鍫ヮ暕閺? 濡偓閺屻儳鏁ら幋?department/team/roles閿涘苯顕悡?config/frontend_access.json 閸氬牆鑻熺憴鍕灟
- 鐟欐帟澹婇崚鍡涘帳閸氬孩婀悽鐔告櫏: 闁插秵鏌婇惂璇茬秿閹存牞鐨?/me 閸掗攱鏌?
- ERP 閺屻儴顕楁稉铏光敄/缁涙盯鈧绱撶敮? 濡偓閺?Bridge 閺勵垰鎯佹潻鎰攽閵嗕共eyword/sku_code 閸欏倹鏆熼妴涓卹idge 閺佺増宓佸┃?
- 娴犺濮熷ù浣芥祮/閺冦儱绻旈梻顕€顣? 濡偓閺屻儰鎹㈤崝锛勫Ц閹降鈧礁浼愭担婊勭ウ闂冭埖顔岄妴浣规綀闂勬劖妲搁崥锔藉姬鐡掕櫕鎼锋担婊嗩洣濮?
DOC

sed "s/{VERSION}/$VERSION/g; s/{GEN_TIME}/$GEN_TIME/g" "$OUTPUT_DIR/API_INTEGRATION_GUIDE.md" > "$OUTPUT_DIR/API_INTEGRATION_GUIDE.md.tmp"
mv "$OUTPUT_DIR/API_INTEGRATION_GUIDE.md.tmp" "$OUTPUT_DIR/API_INTEGRATION_GUIDE.md"

echo "Generated API_USAGE_GUIDE.md and API_INTEGRATION_GUIDE.md in $OUTPUT_DIR"
