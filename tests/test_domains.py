import os
import sys
import unittest
import subprocess
import random
from .common import cli53_cmd

class DomainsTest(unittest.TestCase):
    def _unique_name(self):
        return 'temp%d.com' % random.randint(0, sys.maxsize)

    def test_usage(self):
        assert b'usage' in cli53_cmd('-h')

    def test_create_delete(self):
        name = self._unique_name().encode('utf8')
        cli53_cmd('create', name)
        assert name in cli53_cmd('list')
        cli53_cmd('delete', name)
        assert name not in cli53_cmd('list')
