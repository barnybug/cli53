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

- import from BIND format

- export to BIND format

- create resource records

- delete resource records

- works with BIND format zone files we all know and love - no need to edit
  &lt;ChangeResourceRecordSetsRequest&gt; XML!

- create AWS weighted records

- create AWS Alias records to ELB

- create AWS latency-based routing records

- dynamic record creation for EC2 instances

Installation
------------

You'll need to install pip if you've not installed it already.

Ubuntu systems:

	$ apt-get install python python-pip

Redhat systems (eg Amazon Linux):

	$ yum install python27 python27-pip

Then install cli53:

	$ sudo pip install cli53

Or on Redhat/Amazon Linux:

	$ sudo pip-2.7 install cli53

(You may need to add /usr/local/bin to your $PATH)

You can then run cli53 from your path:

	$ cli53

You need to set your Amazon credentials in the environment as AWS_ACCESS_KEY_ID
and AWS_SECRET_ACCESS_KEY or configure them in ~/.boto. For more information see:
http://code.google.com/p/boto/wiki/BotoConfig

Getting Started
---------------

Create a hosted zone::

	$ cli53 create example.com --comment 'my first zone'

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


Broken CNAME exports (GoDaddy)
------------------------------
Some DNS providers export broken bind files, without the trailing '.'
on CNAME records. This is a requirement for absolute records
(i.e. ones outside of the qualifying domain).

If you see CNAME records being imported to route53 with an extra
mydomain.com on the end (e.g. ghs.google.com.mydomain.com), then you
need to fix your zone file before importing:

        $ perl -pe 's/(CNAME\s+[-a-zA-Z0-9.-_]+)(?!.)$/$1./i' broken.txt > fixed.txt

Dynamic records for EC2 instances
---------------------------------
This functionality allows you to give your EC2 instances memorable DNS names
under your domain. The name will be taken from the 'Name' tag on the instance,
if present, and a CNAME record created pointing to the instance's public DNS
name (ec2-...).

In the instance Name tag, you can either use a partial host name 'app01.prd' or
'app01.prd.mydomain.com' - either creates the correct record.

The CNAME will resolve to the external IP address outside EC2 and to the
internal IP address from another instance inside EC2.

Another feature supported is whilst an instance is stopped, if you specify the
parameter '--off fallback.mydomain.com' you can have the dns name fallback to
another host. As an example, a holding page could be served up from this
indicating the system is off currently.

You can use the '--match' parameter (regular expression) to select a subset of
the instances in the account to apply to.

Generally you'll configure cli53 to run regularly from your crontab like so::

    */5 * * * cli53 instances example.com

This runs every 5 minutes to ensure the records are up to date. When there no
changes, this will purely consist of a call to list the domain and the describe
instances API.

If the account the EC2 instances are in differs from the account the route53
domain is managed under, you can configure the EC2 credentials in a separate
file and pass the parameter '--credentials aws.cfg' in. The credentials file is
of the format::

    [profile prd]
    aws_access_key_id=...
    aws_secret_access_key=...
    region=eu-west-1

    [profile qa]
    aws_access_key_id=...
    aws_secret_access_key=...
    region=eu-west-1

As illustrated above, this also allows you to discover instances from multiple
accounts - for example if you split prd and qa. cli53 will scan all '[profile
...]' sections.

Private/public zones
--------------------
To manage zones that have both a private and a public zone, you must specify the
zone ID instead the domain name, which is ambiguous. This is the 13 character ID
after '/hostedzone/' you can see in the output to 'cli53 list'. eg::

    $ cli53 rrcreate ZZZZZZZZZZZZZ name A 127.0.0.1


Caveats
-------
As Amazon limits operations to a maximum of 100 changes, if you
perform a large operation that changes over 100 resource records it
will be split. An operation that involves deletes, followed by updates
such as an import with --replace will very briefly leave the domain
inconsistent. You have been warned!

Changelog
---------
0.5.2

- A note about new version.

0.5.1

- Set EvaluateTargetHealth to 'true' when creating failover ALIASes

- Raise a ValueError when the type of alias is not of the supported ones

- Restrict the values for the '--failover' argument

- Add retry for convert in which rate limitting can occur.

0.5.0

- Remove 'xml' command. Fixes #99

- Handling throttling gracefully.

- Fixes to tests

- Handle Route53 throttling responses while waiting

- Allow specifying an identifier when delete RRs

- Support failover record types (based on work of Lee-Ming Zen)

- Clarify using Zone ID. Fixes #91.

0.4.4

- instances option (-i) to create internal records (@asmap)

- instances option (-a) to create A records (@asmap)

- Making cli53 importable as python module (@aleszoulek)

- Create DNS records for instances without public addresses (@andrewklau)

0.4.3

- Handle duplicate named instances. Fixes #81

0.4.2

- Revert "Support failover record types" ref #79

0.4.1

- Support failover record types (thanks @leezen)

- Optimize comparisons for speed up 'import --replace'. Thanks to @goekesmi. Fixes #75.

- add required EvaluateTargetHealth element for Alias records (thanks @fitt)

0.4.0

- Improve logging

- Add dynamic EC2 instance registration

- Fix exception on unsupported attributes

- Handle / in zone names for arpa domains. fixes #61.

- Nicer error messages on invalid zone files

- pep8/code formatting

0.3.6

- Support for zone comments

0.3.5

- Fix for zero weighted records

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
