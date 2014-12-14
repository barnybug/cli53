import unittest
import subprocess
import sys
import re
import random
from .common import cli53_cmd

class RegexEqual(object):
    def __init__(self, r):
        self.re = re.compile(r)

    def __eq__(self, x):
        return bool(self.re.search(x))

class CommandsTest(unittest.TestCase):
    def setUp(self):
        # re-use if already created
        self.zone = '%d.example.com' % random.randint(0, sys.maxsize)
        cli53_cmd('create', self.zone, '--comment', 'unittests')

    def tearDown(self):
        # clear up
        cli53_cmd('rrpurge', '--confirm', self.zone)
        cli53_cmd('delete', self.zone)

    def _cmd(self, cmd, *args):
        pargs = ('scripts/cli53', cmd) + args
        p = subprocess.Popen(pargs, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
        p.wait()
        if p.returncode:
            # print >> sys.stderr, p.stderr.read()
            raise NonZeroExit
        return p.stdout.read()

    def test_rrcreate(self):
        cli53_cmd('rrcreate', self.zone, '', 'A', '10.0.0.1')
        cli53_cmd('rrcreate', self.zone, 'www', 'CNAME', self.zone+'.', '-x 3600')
        cli53_cmd('rrcreate', self.zone, 'info', 'TXT', 'this is a "test"')
        cli53_cmd('rrcreate', self.zone, 'weighttest1', 'CNAME', self.zone+'.', '-x 60', '-w 0', '-i awsweightzero')
        cli53_cmd('rrcreate', self.zone, 'weighttest2', 'CNAME', self.zone+'.', '-x 60', '-w 1', '-i awsweightone')
        cli53_cmd('rrcreate', self.zone, 'weighttest3', 'CNAME', self.zone+'.', '-x 60', '-w 50', '-i awsweightfifty')

        output = cli53_cmd('export', self.zone)
        output = [ x for x in output.split(b'\n') if b'10.0.0.1' in x or b'CNAME' in x or b'TXT' in x ]

        self.assertEqual(
            [
                b"@ 86400 IN A 10.0.0.1",
                b'info 86400 IN TXT "this is a \\"test\\""',
                b"weighttest1 60 AWS CNAME 0 " + self.zone.encode('utf8') + b".  awsweightzero",
                b"weighttest2 60 AWS CNAME 1 " + self.zone.encode('utf8') + b".  awsweightone",
                b"weighttest3 60 AWS CNAME 50 " + self.zone.encode('utf8') + b".  awsweightfifty",
                b"www 3600 IN CNAME " + self.zone.encode('utf8') + b".",
            ],
            output
        )

    def test_rrdelete(self):
        cli53_cmd('rrcreate', self.zone, '', 'A', '10.0.0.1')
        cli53_cmd('rrdelete', self.zone, '', 'A')

    def test_rrcreate_replace_latency(self):
        cli53_cmd('rrcreate', '-i', 'asiacdn', '--region', 'ap-southeast-1', self.zone, 'cdn', 'CNAME', 'asiacdn.com.')
        cli53_cmd('rrcreate', '-i', 'statescdn', '--region', 'us-west-1', self.zone, 'cdn', 'CNAME', 'uscdn.com.')
        cli53_cmd('rrcreate', '-i', 'newuscdn', '--region', 'us-west-1', self.zone, 'cdn', 'CNAME', 'newuscdn.com.', '-r')
