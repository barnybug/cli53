cli53 - Command line script to administer the Amazon Route 53 dns service
=========================================================================

Introduction
------------
The latest Amazon service Route 53 is a great addition, but only has a rudimentary set of tools
available at the time of release. This script fills that gap until things have caught up.

Features:
- create hosted zones

- delete hosted zones

- list hosted zones

- import to BIND format

- export to BIND format

- create resource records

- delete resource records

- works with BIND format zone files we all know and love - no need to edit
  <ChangeResourceRecordSetsRequest> XML!

Getting Started
---------------

Create a hosted zone:
    ./cli53.py create example.com

Check what we've done:
    ./cli53.py list

Import a BIND zone file:
    ./cli53.py import example.com --file zonefile.txt

Manually create some records:
    ./cli53.py rrcreate example.com www A 192.168.0.1 --ttl 3600
    ./cli53.py rrcreate example.com www A 192.168.0.2 --ttl 3600 --replace
    ./cli53.py rrcreate example.com '' MX '10 192.168.0.1' '20 192.168.0.2'

Export as a BIND zone file (useful for checking):
    ./cli53.py export example.com

Further documentation is available, e.g.:
    ./cli53.py --help
    ./cli53.py rrcreate --help

Installation
------------
There is no need to install, but you will need python, the latest boto library (from git) and dnspython:

    $ git clone git://github.com/boto/boto && cd boto && python setup.py install
    $ easy_install dnspython

You need to set your Amazon credentials in the environment as AWS_ACCESS_KEY_ID
and AWS_SECRET_ACCESS_KEY.
