#!/bin/bash
export PYTHONPATH=.

COMMENT="unittests${TRAVIS_JOB_ID}"
# Clean up script to run after the tests to ensure any temporary
# domains are deleted to avoid racking up ec2 costs.
scripts/cli53 list | grep -A2 "$COMMENT" | perl -ne 'print "$1\n" if m!Id: /hostedzone/(.+)!' | xargs -n 1 -r scripts/cli53 rrpurge --confirm
scripts/cli53 list | grep -A2 "$COMMENT" | perl -ne 'print "$1\n" if m!Id: /hostedzone/(.+)!' | xargs -n 1 -r scripts/cli53 delete
