import unittest
import subprocess
import sys
import re
import random

class NonZeroExit(Exception):
    pass

class RegexEqual(object):
    def __init__(self, r):
        self.re = re.compile(r)
    
    def __eq__(self, x):
        return bool(self.re.search(x))

class CommandsTest(unittest.TestCase):
    def setUp(self):
        # re-use if already created
        self.zone = '%d.example.com' % random.randint(0, sys.maxint)
        self._cmd('create', self.zone, '--comment', 'unittests')
            
    def tearDown(self):
        # clear up
        self._cmd('rrpurge', '--confirm', self.zone)
        self._cmd('delete', self.zone)
        
    def _cmd(self, cmd, *args):
        pargs = ('scripts/cli53', cmd) + args
        p = subprocess.Popen(pargs, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
        p.wait()
        if p.returncode:
            print >> sys.stderr, p.stderr.read()
            raise NonZeroExit
        return p.stdout.read()
        
    def test_rrcreate(self):
        self._cmd('rrcreate', self.zone, '', 'A', '10.0.0.1')
        self._cmd('rrcreate', self.zone, 'www', 'CNAME', self.zone+'.', '-x 3600')
        self._cmd('rrcreate', self.zone, 'info', 'TXT', 'this is a "test"')
        self._cmd('rrcreate', self.zone, 'weighttest1', 'CNAME', self.zone+'.', '-x 60', '-w 0', '-i awsweightzero')
        self._cmd('rrcreate', self.zone, 'weighttest2', 'CNAME', self.zone+'.', '-x 60', '-w 1', '-i awsweightone')
        self._cmd('rrcreate', self.zone, 'weighttest3', 'CNAME', self.zone+'.', '-x 60', '-w 50', '-i awsweightfifty')

        output = self._cmd('export', self.zone)
        output = [ x for x in output.split('\n') if '10.0.0.1' in x or 'CNAME' in x or 'TXT' in x ]

        self.assertEqual(
            [
                "@ 86400 IN A 10.0.0.1",
                'info 86400 IN TXT "this is a \\"test\\""',
                "weighttest1 60 AWS CNAME 0 %s.  awsweightzero" % self.zone,
                "weighttest2 60 AWS CNAME 1 %s.  awsweightone" % self.zone,
                "weighttest3 60 AWS CNAME 50 %s.  awsweightfifty" % self.zone,
                "www 3600 IN CNAME %s." % self.zone,
            ],
            output
        )

    def test_rrdelete(self):
        self._cmd('rrcreate', self.zone, '', 'A', '10.0.0.1')
        self._cmd('rrdelete', self.zone, '', 'A')
        
    def test_rrcreate_replace_latency(self):
        self._cmd('rrcreate', '-i', 'asiacdn', '--region', 'ap-southeast-1', self.zone, 'cdn', 'CNAME', 'asiacdn.com.')
        self._cmd('rrcreate', '-i', 'statescdn', '--region', 'us-west-1', self.zone, 'cdn', 'CNAME', 'uscdn.com.')
        self._cmd('rrcreate', '-i', 'newuscdn', '--region', 'us-west-1', self.zone, 'cdn', 'CNAME', 'newuscdn.com.', '-r')
