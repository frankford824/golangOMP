import datetime
import io
import json
import os
import re
import sys
import urllib.error
import urllib.request
import uuid
import xml.etree.ElementTree as ET
import zipfile


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, *_args, **_kwargs):
        return None


def run():
    base = os.environ.get("OA_SMOKE_URL", "https://oa.yongbo.cloud").rstrip("/")
    if base not in {"https://oa.yongbo.cloud", "http://127.0.0.1:18088"}:
        raise RuntimeError("OA_SMOKE_URL must be the isolated OA origin")
    admin = os.environ["OA_SMOKE_ADMIN_TOKEN"]
    applicant = os.environ["OA_SMOKE_APPLICANT_TOKEN"]
    hr_id = int(os.environ["OA_SMOKE_HR_ID"])
    new_no = int(os.environ["OA_SMOKE_NEW_EMPLOYEE_NO"])
    marker = os.environ["OA_SMOKE_QA_PREFIX"]
    if not re.fullmatch(r"QA-OA-[A-Za-z0-9-]{3,48}", marker) or not 0 <= new_no <= 9999:
        raise RuntimeError("Invalid QA marker or employee number")
    opener = urllib.request.build_opener(urllib.request.ProxyHandler({}), NoRedirect())

    def request(token, path, method="GET", body=None, allowed=(200,), raw=False, file=None):
        headers = {"Authorization": "Bearer " + token}
        if file is not None:
            name, content_type, content = file
            boundary = uuid.uuid4().hex
            headers["Content-Type"] = "multipart/form-data; boundary=" + boundary
            body = (f'--{boundary}\r\nContent-Disposition: form-data; name="file"; filename="{name}"\r\n'
                    f'Content-Type: {content_type}\r\n\r\n').encode() + content + f"\r\n--{boundary}--\r\n".encode()
        elif body is not None:
            headers["Content-Type"] = "application/json"
            body = json.dumps(body).encode()
        try:
            response = opener.open(urllib.request.Request(base + path, body, headers, method=method), timeout=15)
        except urllib.error.HTTPError as error:
            response = error
        with response:
            if response.status not in allowed:
                raise RuntimeError(f"{method} {path}: HTTP {response.status}, expected {allowed}")
            content = response.read(16 << 20)
        if raw or not content:
            return content
        return json.loads(content)["data"]

    def verify(condition, message):
        if not condition:
            raise RuntimeError(message)

    admin_me = request(admin, "/oa-api/me")
    applicant_me = request(applicant, "/oa-api/me")
    person_path = f"/oa-api/people/{hr_id}"
    person = request(admin, person_path)
    verify(len({admin_me["id"], applicant_me["id"], hr_id}) == 3, "QA accounts must be distinct")
    verify(person["name"].startswith(marker) and applicant_me["name"].startswith(marker)
           and admin_me["name"].startswith(marker), "All identities must bear the run-specific QA prefix")
    verify(person.get("status") == "active" and person.get("employee_no") != new_no,
           "Use a fresh active HR fixture and an unused different employee number")

    workbook = request(admin, "/oa-api/people/export", raw=True)
    revised = qa_roster(workbook, hr_id, new_no, person["name"])
    preview = request(admin, "/oa-api/people/preview", "POST", file=("roster.xlsx", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", revised))
    verify(len(preview["items"]) == 1 and not preview["items"][0]["error"], "HR preview rejected the isolated QA row")
    confirmation = request(admin, "/oa-api/people/confirm", "POST", {"token": preview["token"]})
    verify(confirmation["count"] == 1 and request(admin, person_path)["employee_no"] == new_no, "Employee number did not persist")
    today = datetime.datetime.now(datetime.timezone(datetime.timedelta(hours=8))).date().isoformat()
    for kind, status in [("leave", "inactive"), ("rehire", "active"), ("leave", "inactive")]:
        person = request(admin, person_path)
        changed = request(admin, person_path + "/change", "POST",
                          {"kind": kind, "version": person["version"], "effective_date": today})
        verify(changed["status"] == status, f"Unexpected personnel state after {kind}")
    print("PASS personnel: employee number, leave, rehire, final leave")

    finance = "/oa-api/finance"
    template_id = "qa-" + uuid.uuid4().hex
    template = {"name": marker, "type": "报销", "channels": ["bank"], "fields": ["qa"],
                "required": ["qa"], "evidence": "PDF", "nodes": [f"fixed:{admin_me['id']}"],
                "threshold_cents": 0, "version": 0, "publish": True}
    published = request(admin, finance + "/templates/" + template_id, "PUT", template)
    verify(published["version"] == 1, "Template publication did not create version 1")
    proof = ("qa-proof.pdf", "application/pdf", b"%PDF-1.4\n% QA smoke fixture\n%%EOF\n")
    evidence = request(applicant, finance + "/files", "POST", allowed=(201,), file=proof)
    draft = {"request_id": uuid.uuid4().hex, "template_id": template_id, "amount_cents": 137,
             "reason": marker, "channel": "bank", "holder": marker, "account": "0000000000",
             "bank": "QA fixture", "extra": {"qa": "initial"}, "files": [evidence["id"]], "site": "main"}
    document = request(applicant, finance + "/documents", "POST", draft, allowed=(201,))
    doc_path = finance + "/documents/" + document["id"]
    retried = request(applicant, finance + "/documents", "POST", draft)
    verify(retried["id"] == document["id"], "Duplicate application created another document")

    def decide(current, decision):
        return request(admin, doc_path + "/decision", "POST",
                       {"decision": decision, "comment": marker, "current": current["current"], "version": current["version"]})

    returned = decide(document, "return")
    verify(returned["state"] == "returned", "Expected returned document")
    draft["extra"]["qa"] = "corrected"
    resubmitted = request(applicant, doc_path + "/resubmit", "POST", draft)
    verify(resubmitted["id"] == document["id"] and resubmitted["template"]["version"] == 1,
           "Resubmit lost document identity or locked template")
    approved = decide(resubmitted, "approve")
    verify(approved["state"] == "approved", "Expected approved document")
    receipt = request(admin, finance + "/files", "POST", allowed=(201,), file=proof)
    payment = {"amount_cents": 137, "date": today, "receipt": receipt["id"]}
    first = request(admin, doc_path + "/payment", "POST", payment)
    second = request(admin, doc_path + "/payment", "POST", payment)
    verify(first["state"] == second["state"] == "paid" and first["version"] == second["version"],
           "Repeated payment modified the paid document")
    downloaded = request(applicant, finance + "/files/" + receipt["id"], raw=True)
    verify(downloaded == proof[2], "Receipt did not round-trip")
    report = request(admin, finance + "/report?period=" + today)
    matches = [item for item in report["items"] if item["id"] == document["id"]]
    verify(len(matches) == 1 and matches[0]["payment"]["amount_cents"] == payment["amount_cents"],
           "Report does not contain exactly one payment for this fixture")
    print("PASS finance: publish, submit, return, resubmit, approve, pay, retry, receipt, report")


def qa_roster(data, person_id, employee_no, name):
    ns = "http://schemas.openxmlformats.org/spreadsheetml/2006/main"
    tag = lambda key: f"{{{ns}}}{key}"
    output = io.BytesIO()
    with zipfile.ZipFile(io.BytesIO(data)) as original, zipfile.ZipFile(output, "w", zipfile.ZIP_DEFLATED) as result:
        for filename in original.namelist():
            if filename == "xl/sharedStrings.xml":
                result.writestr(filename, f'<sst xmlns="{ns}" count="0" uniqueCount="0"/>')
                continue
            content = original.read(filename)
            if filename in {"xl/worksheets/sheet1.xml", "xl/worksheets/sheet2.xml"}:
                seed = [("用户ID", "工号", "姓名", "用工类型"), (str(person_id), str(employee_no), name, "")]
                rows = seed if filename.endswith("sheet1.xml") else [("用户ID", "店铺ID", "生效日期", "结束日期")]
                sheet = ET.Element(tag("worksheet"))
                sheet_data = ET.SubElement(sheet, tag("sheetData"))
                for index, fields in enumerate(rows, 1):
                    row = ET.SubElement(sheet_data, tag("row"), {"r": str(index)})
                    for letter, value in zip("ABCD", fields):
                        cell = ET.SubElement(row, tag("c"), {"r": f"{letter}{index}", "t": "inlineStr"})
                        ET.SubElement(ET.SubElement(cell, tag("is")), tag("t")).text = value
                content = ET.tostring(sheet, encoding="utf-8")
            result.writestr(filename, content)
    return output.getvalue()


if __name__ == "__main__":
    try:
        run()
    except (KeyError, ValueError, RuntimeError, OSError) as failure:
        print(f"FAIL: {failure}", file=sys.stderr)
        sys.exit(1)
