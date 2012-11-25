[![Build status](https://secure.travis-ci.org/barnybug/cli53.png?branch=master)](https://secure.travis-ci.org/barnybug/cli53)

cli53 - Command line script to administer the Amazon Route 53 DNS service
=========================================================================

Introduction
------------
cli53 provides import and export from BIND format and simple command line management of
Route 53 domains.

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
  
- create AWS weighted records

- create AWS Alias records to ELB

- create AWS latency-based routing records

Getting Started
---------------

Create a hosted zone::

	$ cli53 create example.com

Check what we've done::

	$ cli53 list

Import a BIND zone file::

	$ cli53 import example.com --file zonefile.txt

Replace with an imported zone, waiting for completion::

	$ cli53 import example.com --file zonefile.txt --replace --wait

Manually create some records::

	$ cli53 rrcreate example.com www A 192.168.0.1 --ttl 3600
	$ cli53 rrcreate example.com www A 192.168.0.2 --ttl 3600 --replace
	$ cli53 rrcreate example.com '' MX '10 192.168.0.1' '20 192.168.0.2'

Export as a BIND zone file (useful for checking)::

	$ cli53 export example.com

Create some weighted records::

	$ cli53 rrcreate example.com www A 192.168.0.1 --weight 10 --identifier server1
	$ cli53 rrcreate example.com www A 192.168.0.2 --weight 20 --identifier server2

Create an alias to ELB::

	$ cli53 rrcreate example.com www ALIAS 'ABCDEFABCDE dns-name.elb.amazonaws.com.'

Further documentation is available, e.g.::

	$ cli53 --help
	$ cli53 rrcreate --help


Installation
------------

	$ sudo pip install cli53

You can then run cli53 from your path:

	$ cli53
 
You need to set your Amazon credentials in the environment as AWS_ACCESS_KEY_ID
and AWS_SECRET_ACCESS_KEY or configure them in ~/.boto. For more information see:
http://code.google.com/p/boto/wiki/BotoConfig

Broken CNAME exports (GoDaddy)
------------------------------
Some DNS providers export broken bind files, without the trailing '.'
on CNAME records. This is a requirement for absolute records
(i.e. ones outside of the qualifying domain).

If you see CNAME records being imported to route53 with an extra
mydomain.com on the end (e.g. ghs.google.com.mydomain.com), then you
need to fix your zone file before importing:

        $ perl -pe 's/(CNAME .+)(?!.)$/$1./i' broken.txt > fixed.txt

Caveats
-------
As Amazon limits operations to a maximum of 100 changes, if you
perform a large operation that changes over 100 resource records it
will be split. An operation that involves deletes, followed by updates
such as an import with --replace will very briefly leave the domain
inconsistent. You have been warned!

Changelog
---------
0.3.3

- Check boto version

0.3.2

- Added functionality to rrlist, rrcreate, import and export so that
  they're able to work with Alias records that have an identifier and
  a latency based or weighted routing policy. (xbe)

- Improve error message when boto fails to import

0.3.1

- Added support for Latency-based routing. For the moment to use this
      you'll need the boto develop branch: pip install
      https://github.com/boto/boto/tarball/develop

0.3.0

- Added support for AWS extensions: weighted records and aliased
  records.
