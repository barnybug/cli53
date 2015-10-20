[![Build status](https://secure.travis-ci.org/barnybug/cli53.png?branch=master)](https://secure.travis-ci.org/barnybug/cli53) [![codecov.io](http://codecov.io/github/barnybug/cli53/coverage.svg?branch=master)](http://codecov.io/github/barnybug/cli53?branch=master)

# cli53 - Command line tool to administer the Amazon Route 53 DNS service

## Introduction

cli53 provides import and export from BIND format and simple command line management of
Route 53 domains.

Features:

- import and export BIND format

- create, delete and list hosted zones

- create, delete and update individual records

- create AWS extensions: failover, geolocation, latency, weighted and ALIAS records

## Installation

Installation is easy, just download the binary from the github releases page (builds are available for Linux, Mac and Windows):
https://github.com/barnybug/cli53/releases/latest

    $ sudo mv cli53-my-platform /usr/local/bin/cli53
    $ sudo chmod +x /usr/local/bin/cli53

To configure your Amazon credentials, either place them in a file `~/.aws/credentials`:

	[default]
	aws_access_key_id = AKID1234567890
	aws_secret_access_key = MY-SECRET-KEY

Or set the environment variables: `AWS_ACCESS_KEY_ID` and `AWS_SECRET_ACCESS_KEY`.

For more information, see: http://blogs.aws.amazon.com/security/post/Tx3D6U6WSFGOK2H/A-New-and-Standardized-Way-to-Manage-Credentials-in-the-AWS-SDKs

## Building from source

To build yourself from source (you will need golang 1.5 installed):

    $ go get github.com/barnybug/cli53
    $ cd $GOPATH/src/github.com/barnybug/cli53
    $ make install

This will produce a binary `cli53` in `~/go/bin`, after this follow the steps as above.

## Getting Started

Create a hosted zone:

	$ cli53 create test.example.com --comment 'my first zone'

Check what we've done:

	$ cli53 list

Import a BIND zone file:

	$ cli53 import --file zonefile.txt test.example.com

Replace with an imported zone, waiting for completion:

	$ cli53 import --file zonefile.txt --replace --wait test.example.com

Manually create some records:

	$ cli53 rrcreate test.example.com 'www 3600 A 192.168.0.1'
	$ cli53 rrcreate --replace test.example.com 'www 3600 A 192.168.0.2'
	$ cli53 rrcreate test.example.com '@ MX "10 192.168.0.1" "20 192.168.0.2"'

Export as a BIND zone file (for backup!):

	$ cli53 export test.example.com

Create some weighted records:

	$ cli53 rrcreate --identifier server1 --weight 10 test.example.com 'www A 192.168.0.1'
	$ cli53 rrcreate --identifier server2 --weight 20 test.example.com 'www A 192.168.0.2'

Create an alias to ELB:

	$ cli53 rrcreate test.example.com 'www ALIAS ABCDEFABCDE dns-name.elb.amazonaws.com.'

Further documentation is available, e.g.:

	$ cli53 --help
	$ cli53 rrcreate --help

## Bug reports

Please open a github issue including cli53 version number `cli53 --version`
and the commands or a zone file to reproduce the issue. A good bug report is
much appreciated!

## Pull requests

Pull requests are gratefully received, though please do include a test case
too.

## Where's python/pypi cli53?

I've since rewritten the original python cli53. As people were still
installing the old version I've taken it off pypi. If you must, you can still
install the python cli53 by giving pip the github branch:

	$ pip install git+https://github.com/barnybug/cli53.git@python

Bare in mind I'll no longer be supporting this any more, so any bug reports
will be flatly closed!

## Broken CNAME exports (GoDaddy)

Some DNS providers export broken bind files, without the trailing '.'
on CNAME records. This is a requirement for absolute records
(i.e. ones outside of the qualifying domain).

If you see CNAME records being imported to route53 with an extra
mydomain.com on the end (e.g. ghs.google.com.mydomain.com), then you
need to fix your zone file before importing:

        $ perl -pe 's/(CNAME\s+[-a-zA-Z0-9.-_]+)(?!.)$/$1./i' broken.txt > fixed.txt

## Private/public zones

To manage zones that have both a private and a public zone, you must specify the
zone ID instead the domain name, which is ambiguous. This is the 13 character ID
after '/hostedzone/' you can see in the output to 'cli53 list'. eg:

    $ cli53 rrcreate ZZZZZZZZZZZZZ 'name A 127.0.0.1'

Caveats
-------
As Amazon limits operations to a maximum of 100 changes, if you
perform a large operation that changes over 100 resource records it
will be split. An operation that involves deletes, followed by updates
such as an import with --replace will very briefly leave the domain
inconsistent. You have been warned!

Changelog
---------
0.6.0 New go version released!
