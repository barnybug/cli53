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
  &lt;ChangeResourceRecordSetsRequest&gt; XML!

Getting Started
---------------

Create a hosted zone:
    ./cli53.py create example.com

Check what we've done:
    ./cli53.py list

Import a BIND zone file:
    ./cli53.py import example.com --file zonefile.txt

Replace with an imported zone, waiting for completion:
    ./cli53.py import example.com --file zonefile.txt --replace --wait

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

### Ubuntu

    $ git clone git://github.com/boto/boto && cd boto && sudo python setup.py install
    $ sudo easy_install dnspython

### CentOS 5.x

There are a couple of extra requirements as CentOS has the older python 2.4:

    $ git clone git://github.com/boto/boto && cd boto && sudo python setup.py install
    $ sudo yum install python-elementtree
    $ sudo easy_install uuid
    $ sudo easy_install dnspython

You need to set your Amazon credentials in the environment as AWS_ACCESS_KEY_ID
and AWS_SECRET_ACCESS_KEY.

Caveats
-------
As Amazon limits operations to a maximum of 100 changes, if you
perform a large operation that changes over 100 resource records it
will be split. An operation that involves deletes, followed by updates
such as an import with --replace will very briefly leave the domain
inconsistent. You have been warned!
