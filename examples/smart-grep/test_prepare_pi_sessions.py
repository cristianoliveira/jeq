import json, os, subprocess, sys, tempfile, unittest
from pathlib import Path
SCRIPT=Path(__file__).with_name('prepare_pi_sessions.py')
class PrepareTests(unittest.TestCase):
 def runp(self, root, *args): return subprocess.run([sys.executable,str(SCRIPT),'--root',str(root),*args],capture_output=True,text=True)
 def test_real_schema_counts_and_redacts(self):
  with tempfile.TemporaryDirectory() as d:
   rows=[{'type':'message','message':{'role':'toolResult','toolName':'bash','isError':True,'content':[{'text':'timeout PRIVATE TOOL /secret/session'}]}},{'type':'message','message':{'role':'assistant','errorMessage':'quota PRIVATE PROVIDER'}}]
   p=Path(d)/'s.jsonl'; p.write_text('\n'.join(json.dumps(row) for row in rows)+'\n'); r=self.runp(Path(d)); self.assertEqual(r.returncode,0); self.assertIn('bash.timeout',r.stdout); self.assertIn('provider.quota',r.stdout); self.assertNotIn('PRIVATE',r.stdout); self.assertNotIn('/secret',r.stdout)
 def test_nested_unrelated_error_is_ignored(self):
  with tempfile.TemporaryDirectory() as d:
   (Path(d)/'s.jsonl').write_text(json.dumps({'type':'message','message':{'role':'assistant','content':[{'errorMessage':'PRIVATE'}]}})); r=self.runp(Path(d)); self.assertEqual(r.returncode,0); self.assertNotIn('provider.',r.stdout)
 def test_malformed_has_no_partial_stdout(self):
  with tempfile.TemporaryDirectory() as d:
   (Path(d)/'bad.jsonl').write_text('{bad\n'); r=self.runp(Path(d)); self.assertEqual(r.returncode,2); self.assertEqual(r.stdout,'')
 def test_order_and_repeat_counts(self):
  with tempfile.TemporaryDirectory() as d:
   for name in ('a.jsonl','b.jsonl'):
    Path(d,name).write_text(json.dumps({'type':'message','message':{'role':'assistant','errorMessage':'quota'}})+'\n')
   r=self.runp(Path(d)); self.assertEqual(r.returncode,0); self.assertIn('2 events across 2 session files, with 0 repeat events',r.stdout)
 def test_caps(self):
  with tempfile.TemporaryDirectory() as d:
   Path(d,'s.jsonl').write_text('{}\n'); r=self.runp(Path(d),'--max-bytes','1'); self.assertEqual(r.returncode,2); self.assertEqual(r.stdout,'')
if __name__=='__main__': unittest.main()
