import json, os, subprocess, sys, tempfile, time, unittest
from pathlib import Path

SCRIPT=Path(__file__).with_name('prepare_pi_sessions.py')
class PrepareTests(unittest.TestCase):
 def runp(self, root, *args): return subprocess.run([sys.executable,str(SCRIPT),'--root',str(root),*args],capture_output=True,text=True)
 def test_redacts_and_counts(self):
  with tempfile.TemporaryDirectory() as d:
   p=Path(d)/'s.jsonl'; p.write_text(json.dumps({'role':'assistant','errorMessage':'PRIVATE PROMPT'})+'\n'+json.dumps({'toolResult':{'isError':True,'content':'PRIVATE TOOL'}})+'\n')
   r=self.runp(Path(d)); self.assertEqual(r.returncode,0); self.assertNotIn('PRIVATE',r.stdout); self.assertIn('assistant_error',r.stdout); self.assertIn('tool_failure',r.stdout); self.assertIn('none',r.stdout)
 def test_malformed_has_no_partial_stdout(self):
  with tempfile.TemporaryDirectory() as d:
   (Path(d)/'bad.jsonl').write_text('{bad\n'); r=self.runp(Path(d)); self.assertEqual(r.returncode,2); self.assertEqual(r.stdout,'')
 def test_mtime_window(self):
  with tempfile.TemporaryDirectory() as d:
   p=Path(d)/'old.jsonl'; p.write_text(json.dumps({'errorMessage':'old'})); os.utime(p,(1,1)); r=self.runp(Path(d),'--since','2','--until','3'); self.assertEqual(r.returncode,0); self.assertIn('none',r.stdout); self.assertNotIn('assistant_error',r.stdout)
if __name__=='__main__': unittest.main()
