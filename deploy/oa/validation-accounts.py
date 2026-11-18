#!/usr/bin/env python3
"""Initialize private reviewer logins in the isolated core; never send to an external origin."""
import json
import os
import secrets
import urllib.error
import urllib.request
from pathlib import Path

base = "http://127.0.0.1:18087"
directory = Path("/etc/yongbo-oa")
creds = directory / "review-accounts.json"
os.umask(0o077)
opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))

def api(path, token=None, body=None, method=None):
    headers = {"Content-Type": "application/json"}
    if token:
        headers["Authorization"] = "Bearer " + token
    req = urllib.request.Request(base+path, None if body is None else json.dumps(body).encode(), headers, method=method)
    try:
        with opener.open(req, timeout=10) as resp:
            raw = resp.read()
            return json.loads(raw)["data"] if raw else None
    except urllib.error.HTTPError as err:
        # The API may include supplied fields in diagnostics; never echo the payload.
        raise RuntimeError(f"{req.method if hasattr(req,'method') else req.get_method()} {path.split('?')[0]} returned {err.code}") from None

def login(account):
    return api("/v1/auth/login", body={"username":account["username"],"password":account["password"]})["session"]["token"]

def main():
    admin={"username":"oa-review-admin","password":(directory/"admin-password").read_text().strip()}
    token=login(admin)
    admin["id"]=api("/v1/auth/me", token)["id"]
    if creds.exists():
        accounts=json.loads(creds.read_text())
    else:
        accounts={}
    accounts["administrator"]=admin
    roles={x["code"]:x["id"] for x in api("/v1/access/roles",token)}
    department=next(d for d in api("/v1/org/options",token)["departments"] if d["enabled"] and d["name"]=="运营部")
    team=next(t["name"] for t in department["team_items"] if t["enabled"])
    active_numbers=set()
    by_username={}
    for page in range(1,101):
        people=api("/v1/users?page_size=100&page="+str(page),token)
        by_username.update((x["username"],x) for x in people)
        active_numbers.update(x.get("employee_no") for x in people)
        if len(people)<100:break
    def persist():
        tmp=creds.with_suffix(".next");tmp.write_text(json.dumps(accounts,ensure_ascii=False),encoding="utf-8");tmp.chmod(0o600);tmp.replace(creds)
    for i,(key,role) in enumerate([("qa-admin","super_admin"),("qa-applicant",None),("qa-hr",None),("review-lead","department_admin"),("review-finance",None),("review-cashier",None),("review-designer","designer"),("review-audit","auditor"),("review-asset","asset_submitter"),("review-settlement","asset_settlement"),("review-asset-manager","asset_manager")]):
        if key not in accounts:
            no=next(x for x in range(9700,9999) if x not in active_numbers)
            active_numbers.add(no)
            accounts[key]={"username":"oa-"+key,"password":secrets.token_urlsafe(24)+"9a","employee_no":no}
            persist()
        acct=accounts[key]
        if not acct.get("id"):
            name="QA-OA-Review "+key
            user=by_username.get(acct["username"])
            if user and user["display_name"]!=name:
                raise RuntimeError("Validation username already belongs to a different person")
            if not user:
                if any(p.get("employee_no")==acct["employee_no"] for p in by_username.values()):
                    acct["employee_no"]=next(x for x in range(9700,9999) if x not in active_numbers)
                active_numbers.add(acct["employee_no"]);persist()
                user=api("/v1/users",token,{"username":acct["username"],"password":acct["password"],"employee_no":acct["employee_no"],"display_name":name,"department":department["name"],"team":team,"mobile":"1390006"+str(9900+i),"status":"active"})
                by_username[user["username"]]=user
            acct["id"]=user["id"];persist()
        if role:
            effective=api(f"/v1/access/users/{acct['id']}/effective",token)
            if not any(row["role_code"]==role for row in effective["assignments"]):
                api(f"/v1/access/users/{acct['id']}/assignments",token,{"expected_policy_revision":effective["policy_revision"],"reason":"OA isolated acceptance accounts","assignments":[{"role_id":roles["member"],"scope_mode":"self","subjects":[]},{"role_id":roles[role],"scope_mode":"own_department" if role=="department_admin" else "global","subjects":[]}]},method="PUT")
    persist()
    print("Validation account preparation complete.")
if __name__ == "__main__":
    main()
