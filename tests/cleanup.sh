#!/usr/bin/bash
export PYTHONPATH=.

# Clean up script to run after the tests to ensure any temporary
# domains are deleted to avoid racking up ec2 costs.
scripts/cli53 list | grep -oP 'Id: /hostedzone/\K.+' | xargs -n 1 scripts/cli53 rrpurge --confirm
scripts/cli53 list | grep -oP 'Id: /hostedzone/\K.+' | xargs -n 1 scripts/cli53 delete
