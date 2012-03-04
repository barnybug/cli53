import unittest
import subprocess
import random

class DomainsTest(unittest.TestCase):
    def _cmd(self, cmd, *args):
        pargs = ('scripts/cli53', cmd) + args
        return subprocess.check_output(pargs, stderr=subprocess.STDOUT)
        
    def _unique_name(self):
        return 'temp%d.com' % random.randint(0, 65535)
        
    def test_usage(self):
        assert 'usage' in self._cmd('-h')        

    def test_create_delete(self):
        name = self._unique_name()
        self._cmd('create', name)
        assert name in self._cmd('list')
        self._cmd('delete', name)
        assert name not in self._cmd('list')
