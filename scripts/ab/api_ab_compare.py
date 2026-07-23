#!/usr/bin/env python3
"""GET-only, all-task, fail-closed A/B API comparator for local clones."""
from __future__ import annotations
import argparse, hashlib, json, pathlib, re, urllib.error, urllib.parse, urllib.request

ENDPOINTS=("/v1/tasks/{id}","/v1/tasks/{id}/detail","/v1/tasks/{id}/events","/v1/tasks/{id}/resource-bundle")

def canonical(v: object) -> bytes: return (json.dumps(v,ensure_ascii=False,sort_keys=True,separators=(',',':'))+'\n').encode()
def load_tasks(path: pathlib.Path) -> list[str]:
    ids=[]
    for n,line in enumerate(path.read_text(encoding='utf-8').splitlines(),1):
        line=line.strip()
        if not line: continue
        try:
            value=json.loads(line); value=value.get('task_id') if isinstance(value,dict) else value
        except json.JSONDecodeError: value=line.split(',')[0]
        value=str(value)
        if not re.fullmatch(r'[1-9][0-9]*',value): raise ValueError(f'invalid task id on line {n}')
        ids.append(value)
    if not ids or len(ids)!=len(set(ids)): raise ValueError('task list must be non-empty and unique')
    return ids
def headers(path: pathlib.Path) -> dict[str,str]:
    data=json.loads(path.read_text(encoding='utf-8')) if path else {}
    if not isinstance(data,dict) or not all(isinstance(k,str) and isinstance(v,str) for k,v in data.items()): raise ValueError('headers file must be a string map')
    return data
def local_url(url: str) -> str:
    parsed=urllib.parse.urlparse(url)
    if parsed.scheme not in {'http','https'} or parsed.hostname not in {'127.0.0.1','localhost','host.docker.internal'}: raise ValueError('API clone URL must be local')
    return url.rstrip('/')
def get(base: str,path: str,hdr: dict[str,str]) -> tuple[int,object,bytes]:
    req=urllib.request.Request(base+path,headers=hdr,method='GET')
    try:
        with urllib.request.urlopen(req,timeout=20) as res: status=res.status; raw=res.read()
    except urllib.error.HTTPError as exc: status=exc.code; raw=exc.read()
    if status>=500: raise ValueError(f'GET {path} returned {status}')
    ctype=''
    try: ctype=req.headers.get('Content-Type','')
    except Exception: pass
    try: body=json.loads(raw) if raw else None
    except json.JSONDecodeError: body={'_non_json_sha256':hashlib.sha256(raw).hexdigest(),'_bytes':len(raw),'_content_type':ctype}
    return status,body,raw
def normalize(value: object,rules: dict,path='') -> object:
    ignores=[re.compile(x) for x in rules.get('ignore_paths',[])]
    maps=[(re.compile(x['path_regex']),x.get('map',{})) for x in rules.get('value_maps',[])]
    if any(r.fullmatch(path) for r in ignores): return None
    if isinstance(value,dict):
        return {k:normalize(v,rules,path+'/'+k.replace('~','~0').replace('/','~1')) for k,v in sorted(value.items()) if not any(r.fullmatch(path+'/'+k.replace('~','~0').replace('/','~1')) for r in ignores)}
    if isinstance(value,list): return [normalize(v,rules,path+f'/{i}') for i,v in enumerate(value)]
    for pattern,mapping in maps:
        if pattern.fullmatch(path) and str(value) in mapping: return mapping[str(value)]
    return value
def group_ids(value: object) -> set[str]:
    found=set()
    if isinstance(value,dict):
        if 'scope_kind' in value and isinstance(value.get('id'),int): found.add(str(value['id']))
        if isinstance(value.get('group_id'),int): found.add(str(value['group_id']))
        for child in value.values(): found |= group_ids(child)
    elif isinstance(value,list):
        for child in value: found |= group_ids(child)
    return found
def fetch_history(base,gid,hdr):
    pages=[]; page=1
    while True:
        status,body,raw=get(base,f'/v1/resource-groups/{gid}/revisions?page={page}&page_size=200',hdr)
        if status!=200: return status,body,raw
        pages.append(body)
        data=body.get('data',body) if isinstance(body,dict) else {}
        total=data.get('total',0) if isinstance(data,dict) else 0
        items=data.get('items',[]) if isinstance(data,dict) else []
        if page*200>=total or not items: return 200,pages,canonical(pages)
        page+=1
        if page>10000: raise ValueError(f'history pagination did not terminate for group {gid}')
def manifest_task_ids(path: pathlib.Path, run_id: str) -> set[str]:
    found=set()
    for line in path.read_text(encoding='utf-8').splitlines():
        if not line: continue
        row=json.loads(line)
        if row.get('run_id')==run_id and row.get('gate_name')=='G01' and str(row.get('entity_key','')).startswith('task:'):
            found.add(str(row['entity_key']).split(':',1)[1])
    if not found: raise ValueError('reviewed manifest has no G01 task entities')
    return found
def compare(args) -> dict:
    base_a=local_url(args.api_a_url); base_b=local_url(args.api_b_url)
    if base_a==base_b: raise ValueError('A and B API URLs must differ')
    rules=json.loads(args.rules.read_text(encoding='utf-8')); ha=headers(args.headers_a); hb=headers(args.headers_b)
    out=args.output_dir; (out/'raw/a').mkdir(parents=True); (out/'raw/b').mkdir(parents=True)
    violations=[]; requests=0; tasks=load_tasks(args.task_ids)
    manifest_tasks=manifest_task_ids(args.manifest,args.run_id)
    if set(tasks)!=manifest_tasks: raise ValueError(f'task list does not exactly match reviewed manifest: list={len(tasks)},manifest={len(manifest_tasks)}')
    for task in tasks:
        bundles={}
        for endpoint in ENDPOINTS:
            path=endpoint.format(id=task); sa,ba,ra=get(base_a,path,ha); sb,bb,rb=get(base_b,path,hb); requests+=2
            slug=path.strip('/').replace('/','_')
            (out/'raw/a'/f'{slug}.json').write_bytes(ra); (out/'raw/b'/f'{slug}.json').write_bytes(rb)
            allowed={(int(x[0]),int(x[1])) for x in rules.get('allowed_status_pairs',{}).get(endpoint,[])}
            if sa!=sb and (sa,sb) not in allowed: violations.append({'violation_code':'api.status_mismatch','entity_key':f'{task}:{endpoint}','detail':f'A={sa},B={sb}'})
            elif sa==sb==200 and canonical(normalize(ba,rules))!=canonical(normalize(bb,rules)):
                violations.append({'violation_code':'api.body_mismatch','entity_key':f'{task}:{endpoint}','detail':f'A={hashlib.sha256(canonical(normalize(ba,rules))).hexdigest()},B={hashlib.sha256(canonical(normalize(bb,rules))).hexdigest()}'})
            if endpoint.endswith('resource-bundle'): bundles={'a':ba if sa==200 else {},'b':bb if sb==200 else {}}
        for gid in sorted(group_ids(bundles.get('a',{}))|group_ids(bundles.get('b',{})),key=int):
            sa,ba,ra=fetch_history(base_a,gid,ha); sb,bb,rb=fetch_history(base_b,gid,hb); requests+=2
            (out/'raw/a'/f'group_{gid}_revisions.json').write_bytes(ra); (out/'raw/b'/f'group_{gid}_revisions.json').write_bytes(rb)
            endpoint='/v1/resource-groups/{group_id}/revisions'
            allowed={(int(x[0]),int(x[1])) for x in rules.get('allowed_status_pairs',{}).get(endpoint,[])}
            if sa!=sb and (sa,sb) not in allowed: violations.append({'violation_code':'api.status_mismatch','entity_key':f'{task}:group:{gid}','detail':f'A={sa},B={sb}'})
            elif sa==sb==200 and canonical(normalize(ba,rules))!=canonical(normalize(bb,rules)):
                violations.append({'violation_code':'api.body_mismatch','entity_key':f'{task}:group:{gid}','detail':'normalized revision histories differ'})
    return {'violation_count':len(violations),'violations':violations,'task_count':len(tasks),'request_count':requests,
            'rules_sha256':hashlib.sha256(args.rules.read_bytes()).hexdigest(),'manifest_sha256':hashlib.sha256(args.manifest.read_bytes()).hexdigest()}
def main():
    p=argparse.ArgumentParser(); p.add_argument('--api-a-url',required=True); p.add_argument('--api-b-url',required=True)
    p.add_argument('--task-ids',type=pathlib.Path,required=True); p.add_argument('--rules',type=pathlib.Path,required=True)
    p.add_argument('--manifest',type=pathlib.Path,required=True); p.add_argument('--run-id',required=True)
    p.add_argument('--headers-a',type=pathlib.Path); p.add_argument('--headers-b',type=pathlib.Path)
    p.add_argument('--output-dir',type=pathlib.Path,required=True); a=p.parse_args(); a.output_dir.mkdir(parents=True,exist_ok=True)
    try: result=compare(a)
    except (OSError,ValueError,json.JSONDecodeError,urllib.error.URLError) as exc: result={'violation_count':1,'violations':[{'violation_code':'api.comparison_error','entity_key':'*','detail':str(exc)}]}
    (a.output_dir/'api-comparison.json').write_bytes(canonical(result))
    raise SystemExit(0 if result['violation_count']==0 else 1)
if __name__=='__main__': main()
