import unittest
import subprocess
import sys
import os
import re
import random
from .common import cli53_cmd, NonZeroExit

def _f(x):
    return os.path.join(os.path.dirname(__file__), x)

class RegexEqual(object):
    def __init__(self, r):
        self.re = re.compile(r)

    def __eq__(self, x):
        return bool(self.re.search(x))

class BindTest(unittest.TestCase):
    def setUp(self):
        cli53_cmd('create', self.zone)

    def tearDown(self):
        # clear up
        cli53_cmd('rrpurge', '--confirm', self.zone)
        cli53_cmd('delete', self.zone)
        os.unlink('temp.txt')

    def _zonefile(self, fname):
        with open('temp.txt', 'w') as fout:
            fout.write("$ORIGIN %s.\n" % self.zone)
            with open(_f(fname), 'r') as fin:
                fout.write(fin.read())
        return 'temp.txt'

class ZoneTest(BindTest):
    zone = '%d.example.com' % random.randint(0, sys.maxsize)

    def test_import(self):
        fname = self._zonefile('zone1.txt')
        cli53_cmd('import', '--file', fname, self.zone)

        output = cli53_cmd('export', self.zone)
        output = [ x for x in output.split(b'\n') if x ]
        output.sort()

        self.assertEqual(
            [
                b"$ORIGIN " + self.zone.encode('utf8') + b'.',
                RegexEqual(b'^@ 172800 IN NS'),
                RegexEqual(b'^@ 172800 IN NS'),
                RegexEqual(b'^@ 172800 IN NS'),
                RegexEqual(b'^@ 172800 IN NS'),
                b"@ 86400 IN A 10.0.0.1",
                b"@ 86400 IN MX 10 mail.example.com.",
                b"@ 86400 IN MX 20 mail2.example.com.",
                b"@ 86400 IN TXT \"v=spf1 a mx a:cli53.example.com mx:mail.example.com ip4:10.0.0.0/24 ~all\"",
                RegexEqual(b'^@ 900 IN SOA'),
                b"mail 86400 IN A 10.0.0.2",
                b"mail2 86400 IN A 10.0.0.3",
                b'test 86400 IN TXT "multivalued" " txt \\"quoted\\" record"',
                b"www 86400 IN A 10.0.0.1",
            ],
            output
        )

    def test_import2(self):
        fname = self._zonefile('zone2.txt')
        cli53_cmd('import', '--file', fname, self.zone)

        output = cli53_cmd('export', self.zone)
        output = [ x for x in output.split(b'\n') if x ]
        output.sort()

        self.assertEqual(
            [
                b"$ORIGIN " + self.zone.encode('utf8') + b'.',
                RegexEqual(b'^@ 172800 IN NS'),
                RegexEqual(b'^@ 172800 IN NS'),
                RegexEqual(b'^@ 172800 IN NS'),
                RegexEqual(b'^@ 172800 IN NS'),
                b"@ 86400 IN A 10.0.0.1",
                b"@ 86400 IN MX 10 mail.example.com.",
                b"@ 86400 IN MX 20 mail2.example.com.",
                b"@ 86400 IN TXT \"v=spf1 a mx a:cli53.example.com mx:mail.example.com ip4:10.0.0.0/24 ~all\"",
                RegexEqual(b'^@ 900 IN SOA'),
                b"mail 86400 IN A 10.0.0.2",
                b"mail2 86400 IN A 10.0.0.3",
                b'test 86400 IN TXT "multivalued" " txt \\"quoted\\" record"',
                b"www 86400 IN A 10.0.0.1",
            ],
            output
        )

    def disabled_aws_extensions(self):
        # disabled - they require a valid ELB to point to
        fname = self._zonefile('zoneaws.txt')
        cli53_cmd('import', '--file', fname, self.zone)

        output = cli53_cmd('export', self.zone)
        output = [ x for x in output.split(b'\n') if x ]
        output.sort()

        self.assertEqual(
            [
                b"$ORIGIN " + self.zone.encode('utf8') + b'.',
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

    def test_invalid1(self):
        fname = self._zonefile('invalid1.txt')
        self.assertRaises(NonZeroExit,
            cli53_cmd, 'import', '--file', fname, self.zone)

def random_arpa_address():
    p = tuple(random.randint(0, 255) for x in range(3))
    return '0/%d.%d.%d.10.in-addr.arpa' % p

class ArpaTest(BindTest):
    zone = random_arpa_address()

    def test_import_arpa(self):
        fname = self._zonefile('zone3.txt')
        cli53_cmd('import', '--file', fname, self.zone)

        output = cli53_cmd('export', self.zone)
        output = [ x for x in output.split(b'\n') if x ]
        output.sort()

        self.assertEqual(
            [
                b"$ORIGIN " + self.zone.encode('utf8') + b'.',
                b"98 0 IN PTR blah.foo.com.",
                RegexEqual(b'^@ 172800 IN NS'),
                RegexEqual(b'^@ 172800 IN NS'),
                RegexEqual(b'^@ 172800 IN NS'),
                RegexEqual(b'^@ 172800 IN NS'),
                RegexEqual(b'^@ 900 IN SOA'),
            ],
            output
        )
