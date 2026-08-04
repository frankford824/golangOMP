import json, pathlib, sys, tempfile, unittest
sys.path.insert(0, str(pathlib.Path(__file__).parent))
from render_evidence import compare, gate_report, mark_blocked, render, split_markers
class RenderTest(unittest.TestCase):
 def test_sort_and_compare(self):
  with tempfile.TemporaryDirectory() as d:
   p=pathlib.Path(d); t=p/'x.tsv'; t.write_text('violation_code\tentity_key\tdetail\nb\t2\tz\na\t1\ty\n')
   render(t,p/'x.csv',p/'x.json'); compare(p/'x.json',p/'x.json',p/'p.json')
   self.assertEqual(json.loads((p/'p.json').read_text())['violation_count'],0)
 def test_split_markers_preserves_headers_and_empty_gate(self):
  with tempfile.TemporaryDirectory() as d:
   p=pathlib.Path(d); combined=p/'combined.tsv'
   combined.write_text('ab_gate_marker\n__AB_GATE__00_snapshot_fingerprint\nmetric\tvalue\ntasks\t2\nab_gate_marker\n__AB_GATE__01_task_state_parity\nviolation_code\tentity_key\tdetail\n')
   split_markers(combined,p/'out')
   self.assertEqual((p/'out/00_snapshot_fingerprint.tsv').read_text(),'metric\tvalue\ntasks\t2\n')
   self.assertEqual((p/'out/01_task_state_parity.tsv').read_text(),'violation_code\tentity_key\tdetail\n')
 def test_split_markers_synthesizes_known_zero_row_gate_headers(self):
  with tempfile.TemporaryDirectory() as d:
   p=pathlib.Path(d); combined=p/'combined.tsv'
   combined.write_text('ab_gate_marker\n__AB_GATE__03_revision_chain\nab_gate_marker\n__AB_GATE__12_legacy_timestamp_contract\n')
   split_markers(combined,p/'out')
   expected='violation_code\tentity_key\tdetail\n'
   self.assertEqual((p/'out/03_revision_chain.tsv').read_text(),expected)
   self.assertEqual((p/'out/12_legacy_timestamp_contract.tsv').read_text(),expected)
 def test_split_markers_rejects_unknown_empty_gate(self):
  with tempfile.TemporaryDirectory() as d:
   p=pathlib.Path(d); combined=p/'combined.tsv'
   combined.write_text('ab_gate_marker\n__AB_GATE__99_unknown\n')
   with self.assertRaises(SystemExit): split_markers(combined,p/'out')
 def test_evidence_rows_are_hashed_but_not_counted_as_violations(self):
  with tempfile.TemporaryDirectory() as d:
   p=pathlib.Path(d); t=p/'events.tsv'
   t.write_text('violation_code\tentity_key\tdetail\nevidence.task_event_log_row\t1:1\tabc\n')
   render(t,p/'events.csv',p/'events.json')
   payload=json.loads((p/'events.json').read_text())
   self.assertEqual(payload['violation_count'],0)
   self.assertEqual(len(payload['evidence']),1)
 def test_empty_output_is_rejected(self):
  with tempfile.TemporaryDirectory() as d:
   p=pathlib.Path(d); t=p/'empty.tsv'; t.write_text('')
   with self.assertRaises(SystemExit): render(t,p/'empty.csv',p/'empty.json')
 def test_mark_blocked_is_fail_closed(self):
  with tempfile.TemporaryDirectory() as d:
   p=pathlib.Path(d); evidence=p/'gate.json'
   evidence.write_text('{"violation_count":0,"violations":[],"evidence":[]}\n')
   mark_blocked(evidence,'manifest.not_consumed','hard_blocked')
   self.assertEqual(json.loads(evidence.read_text())['violation_count'],1)
 def test_gate_report_requires_all_gates_and_parity(self):
  with tempfile.TemporaryDirectory() as d:
   p=pathlib.Path(d); a=p/'a'; b=p/'b'; parity=p/'parity'; a.mkdir(); b.mkdir(); parity.mkdir()
   for index in range(13):
    name=f'{index:02d}_gate'
    payload={'schema_version':1,'status':'PASS','violation_count':0,'violations':[],'evidence':[]}
    (a/f'{name}.json').write_text(json.dumps(payload))
    (b/f'{name}.json').write_text(json.dumps(payload))
   compare(a/'07_gate.json',b/'07_gate.json',parity/'07_event_history_checksum.json')
   payload=gate_report('run-1',a,b,parity,p/'gate_report.json')
   self.assertEqual(payload['status'],'PASS')
   self.assertEqual(len(payload['gates']),13)
