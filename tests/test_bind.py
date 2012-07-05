import unittest
import subprocess
import sys
import os
import re

def _f(x):
    return os.path.join(os.path.dirname(__file__), x)

class NonZeroExit(Exception):
    pass

class RegexEqual(object):
    def __init__(self, r):
        self.re = re.compile(r)
    
    def __eq__(self, x):
        return bool(self.re.search(x))

class BindTest(unittest.TestCase):
    def setUp(self):
        # re-use if already created
        self.zone = 'cli53.example.com'
        try:
            self._cmd('rrpurge', '--confirm', self.zone)
        except NonZeroExit:
            # domain does not exist
            self._cmd('create', self.zone)
            
    def tearDown(self):
        # clear up
        self._cmd('rrpurge', '--confirm', self.zone)
        
    def _cmd(self, cmd, *args):
        pargs = ('scripts/cli53', cmd) + args
        p = subprocess.Popen(pargs, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
        p.wait()
        if p.returncode:
            print >> sys.stderr, p.stderr.read()
            raise NonZeroExit
        return p.stdout.read()
        
    def test_import(self):
        fname = _f('zone1.txt')
        self._cmd('import', '--file', fname, self.zone)
        
        output = self._cmd('export', self.zone)
        output = [ x for x in output.split('\n') if x ]
        output.sort()
        
        self.assertEqual(
            [
                "$ORIGIN cli53.example.com.",
                RegexEqual('^@ 172800 IN NS'),
                RegexEqual('^@ 172800 IN NS'),
                RegexEqual('^@ 172800 IN NS'),
                RegexEqual('^@ 172800 IN NS'),
                "@ 86400 IN A 10.0.0.1",
                "@ 86400 IN MX 10 mail.cli53.example.com.cli53.example.com.",
                "@ 86400 IN MX 20 mail2.cli53.example.com.cli53.example.com.",
                "@ 86400 IN TXT \"v=spf1 a mx a:cli53.example.com mx:mail.cli53.example.com ip4:10.0.0.0/24 ~all\"",
                RegexEqual('^@ 900 IN SOA'),
                "mail 86400 IN A 10.0.0.2",
                "mail2 86400 IN A 10.0.0.3",
                'test 86400 IN TXT "multivalued" " txt \\"quoted\\" record"',
                "www 86400 IN A 10.0.0.1",
            ],
            output
        )

    def test_import2(self):
        fname = _f('zone2.txt')
        self._cmd('import', '--file', fname, self.zone)
        
        output = self._cmd('export', self.zone)
        output = [ x for x in output.split('\n') if x ]
        output.sort()
        
        self.assertEqual(
            [
                "$ORIGIN cli53.example.com.",
                RegexEqual('^@ 172800 IN NS'),
                RegexEqual('^@ 172800 IN NS'),
                RegexEqual('^@ 172800 IN NS'),
                RegexEqual('^@ 172800 IN NS'),
                "@ 86400 IN A 10.0.0.1",
                "@ 86400 IN MX 10 mail.cli53.example.com.cli53.example.com.",
                "@ 86400 IN MX 20 mail2.cli53.example.com.cli53.example.com.",
                "@ 86400 IN TXT \"v=spf1 a mx a:cli53.example.com mx:mail.cli53.example.com ip4:10.0.0.0/24 ~all\"",
                RegexEqual('^@ 900 IN SOA'),
                "mail 86400 IN A 10.0.0.2",
                "mail2 86400 IN A 10.0.0.3",
                'test 86400 IN TXT "multivalued" " txt \\"quoted\\" record"',
                "www 86400 IN A 10.0.0.1",
            ],
            output
        )
        
    def test_aws_extensions(self):
        fname = _f('zoneaws.txt')
        self._cmd('import', '--file', fname, self.zone)
        
        output = self._cmd('export', self.zone)
        output = [ x for x in output.split('\n') if x ]
        output.sort()
        print output
        
        self.assertEqual(
            [
                "$ORIGIN cli53.example.com.",
                RegexEqual('^@ 172800 IN NS'),
                RegexEqual('^@ 172800 IN NS'),
                RegexEqual('^@ 172800 IN NS'),
                RegexEqual('^@ 172800 IN NS'),
                RegexEqual('^@ 900 IN SOA'),
                "test 86400 AWS A 10 127.0.0.1 abc",
                "test 86400 AWS A 20 127.0.0.2 def",
                "test2 600 AWS ALIAS Z3NF1Z3NOM5OY2 test-212960849.eu-west-1.elb.amazonaws.com.",
		"test3 600 AWS ALIAS region:us-west-1 Z3NF1Z3NOM5OY2 test-212960849.eu-west-1.elb.amazonaws.com. identifier-test-id",
		"test4 600 AWS ALIAS 50 Z3NF1Z3NOM5OY2 test-212960849.eu-west-1.elb.amazonaws.com. latency-test-id",
            ],
            output
        )
