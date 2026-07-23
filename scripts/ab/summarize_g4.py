#!/usr/bin/env python3
from __future__ import annotations
import argparse, csv, json, pathlib

def main() -> None:
    p=argparse.ArgumentParser()
    p.add_argument('--steps',type=pathlib.Path,required=True); p.add_argument('--total-seconds',type=int,required=True)
    p.add_argument('--max-step',type=int,required=True); p.add_argument('--max-total',type=int,required=True)
    p.add_argument('--snapshot',type=pathlib.Path,required=True); p.add_argument('--output',type=pathlib.Path,required=True)
    a=p.parse_args(); violations=[]
    with a.steps.open(encoding='utf-8',newline='') as fh: rows=list(csv.DictReader(fh,delimiter='\t'))
    required=['dry_run_before','apply','idempotent_apply','validate_after_apply','rollback','validate_after_rollback']
    if [r.get('step') for r in rows] != required:
        violations.append({'violation_code':'g4.step_sequence','entity_key':'steps','detail':str([r.get('step') for r in rows])})
    for row in rows:
        if int(row['exit_code']) != 0: violations.append({'violation_code':'g4.step_failed','entity_key':row['step'],'detail':row['exit_code']})
        if int(row['elapsed_seconds']) > a.max_step: violations.append({'violation_code':'g4.step_timeout','entity_key':row['step'],'detail':row['elapsed_seconds']})
    if a.total_seconds > a.max_total: violations.append({'violation_code':'g4.total_timeout','entity_key':'total','detail':str(a.total_seconds)})
    try:
        snapshot=json.loads(a.snapshot.read_text(encoding='utf-8'))
        if snapshot.get('apply_state') != 'rolled_back': violations.append({'violation_code':'g4.rollback_state','entity_key':'snapshot','detail':str(snapshot.get('apply_state'))})
        if not snapshot.get('integrity_sha256'): violations.append({'violation_code':'g4.snapshot_integrity_missing','entity_key':'snapshot','detail':'empty integrity_sha256'})
    except (OSError,json.JSONDecodeError) as exc: violations.append({'violation_code':'g4.snapshot_unreadable','entity_key':'snapshot','detail':str(exc)})
    result={'violation_count':len(violations),'violations':violations,'total_seconds':a.total_seconds,'steps':rows}
    a.output.write_text(json.dumps(result,ensure_ascii=False,sort_keys=True,separators=(',',':'))+'\n',encoding='utf-8')
    raise SystemExit(0 if not violations else 1)
if __name__=='__main__': main()
