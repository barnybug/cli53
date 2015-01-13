import os
import subprocess

class NonZeroExit(Exception):
    pass

def cli53_cmd(cmd, *args):
    pargs = ('scripts/cli53', cmd) + args
    env = os.environ
    env['PYTHONPATH'] = '.'
    p = subprocess.Popen(pargs, stdout=subprocess.PIPE, stderr=subprocess.PIPE,
                         env=env)
    p.wait()
    if p.returncode:
        raise NonZeroExit(p.stderr.read())
    return p.stdout.read()
