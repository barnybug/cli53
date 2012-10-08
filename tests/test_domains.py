import unittest
import subprocess
import random

# copied from python 2.7 for python 2.6
def check_output(*popenargs, **kwargs):
    if 'stdout' in kwargs:
        raise ValueError('stdout argument not allowed, it will be overridden.')
    process = subprocess.Popen(stdout=subprocess.PIPE, *popenargs, **kwargs)
    output, unused_err = process.communicate()
    retcode = process.poll()
    if retcode:
        cmd = kwargs.get("args")
        if cmd is None:
            cmd = popenargs[0]
        raise subprocess.CalledProcessError(retcode, cmd, output=output)
    return output

class DomainsTest(unittest.TestCase):
    def _cmd(self, cmd, *args):
        pargs = ('scripts/cli53', cmd) + args
        return check_output(pargs, stderr=subprocess.STDOUT)
        
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
