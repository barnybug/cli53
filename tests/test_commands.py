import os
import unittest
import subprocess
import sys
import re
import random
from common import cli53_cmd, NonZeroExit

class RegexEqual(object):
    def __init__(self, r):
        self.re = re.compile(r)

    def __eq__(self, x):
        return bool(self.re.search(x))

class CommandsTest(unittest.TestCase):
    def setUp(self):
        # re-use if already created
        self.zone = '%d.example.com' % random.randint(0, sys.maxint)

        comment = 'unittests%s' % os.getenv('TRAVIS_JOB_ID', '')
        output = cli53_cmd('create', self.zone, '--comment', comment)
        # extract the zone id
        self.zoneid = [x for x in output.split('\n') if '/hostedzone/' in x][0].split('/')[2]

    def tearDown(self):
        # clear up
        cli53_cmd('rrpurge', '--confirm', self.zone)
        cli53_cmd('delete', self.zone)

    def test_rrcreate(self):
        cli53_cmd('rrcreate', self.zone, '', 'A', '10.0.0.1')
        cli53_cmd('rrcreate', self.zone, 'www', 'CNAME', self.zone + '.', '-x 3600')
        cli53_cmd('rrcreate', self.zone, 'info', 'TXT', 'this is a "test"')
        cli53_cmd('rrcreate', self.zone, 'weighttest1', 'CNAME', self.zone + '.', '-x 60', '-w 0', '-i awsweightzero')
        cli53_cmd('rrcreate', self.zone, 'weighttest2', 'CNAME', self.zone + '.', '-x 60', '-w 1', '-i awsweightone')
        cli53_cmd('rrcreate', self.zone, 'weighttest3', 'CNAME', self.zone + '.', '-x 60', '-w 50', '-i awsweightfifty')

        output = cli53_cmd('export', self.zone)
        output = [x for x in output.split('\n') if '10.0.0.1' in x or 'CNAME' in x or 'TXT' in x]

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

    @unittest.expectedFailure
    def test_rrcreate_no_alias_type(self):
        cli53_cmd('rrcreate', self.zone, 'bad', 'ALIAS', '10.0.0.1', '-i bad')

    @unittest.expectedFailure
    def test_rrcreate_failover_bad_value(self):
        cli53_cmd('rrcreate', self.zone, 'bad', 'A', '10.0.0.1', '-i bad', '--failover', 'BADVALUE')

    def test_rrcreate_failover_ALIAS(self):
        cli53_cmd('rrcreate', self.zone, '', 'A', '10.0.0.1')
        cli53_cmd('rrcreate', self.zone, 'primary', 'ALIAS', "%s %s." % (self.zoneid, self.zone), '-x 60', '-i failover-primary', '--failover', 'PRIMARY')
        cli53_cmd('rrcreate', self.zone, 'secondary', 'ALIAS', "%s %s." % (self.zoneid, self.zone), '-x 60', '-i failover-secondary', '--failover', 'SECONDARY')

        output = cli53_cmd('export', self.zone)
        output = [x for x in output.split('\n') if 'failover-' in x]

        # ALIAS doesn't use TTL in this case, so the value is arbitrary
        self.assertEqual(
            [
                RegexEqual("primary \d+ AWS ALIAS failover:PRIMARY %s %s.  failover-primary" % (self.zoneid, self.zone)),
                RegexEqual("secondary \d+ AWS ALIAS failover:SECONDARY %s %s.  failover-secondary" % (self.zoneid, self.zone)),
            ],
            output
        )

    def test_rrdelete(self):
        cli53_cmd('rrcreate', self.zone, '', 'A', '10.0.0.1')
        cli53_cmd('rrdelete', self.zone, '', 'A')

    def test_rrcreate_replace_latency(self):
        cli53_cmd('rrcreate', '-i', 'asiacdn', '--region', 'ap-southeast-1', self.zone, 'cdn', 'CNAME', 'asiacdn.com.')
        cli53_cmd('rrcreate', '-i', 'statescdn', '--region', 'us-west-1', self.zone, 'cdn', 'CNAME', 'uscdn.com.')
        cli53_cmd('rrcreate', '-i', 'newuscdn', '--region', 'us-west-1', self.zone, 'cdn', 'CNAME', 'newuscdn.com.',
                  '-r')
