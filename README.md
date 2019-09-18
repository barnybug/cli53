[![Build status](https://secure.travis-ci.org/barnybug/cli53.svg?branch=master)](https://secure.travis-ci.org/barnybug/cli53) [![codecov.io](http://codecov.io/github/barnybug/cli53/coverage.svg?branch=master)](http://codecov.io/github/barnybug/cli53?branch=master)
[![Join the chat at https://gitter.im/barnybug/cli53](https://badges.gitter.im/Join%20Chat.svg)](https://gitter.im/barnybug/cli53?utm_source=badge&utm_medium=badge&utm_campaign=pr-badge&utm_content=badge)

# cli53 - Command line tool for Amazon Route 53

## Introduction

cli53 provides import and export from BIND format and simple command line management of
Route 53 domains.

Features:

- import and export BIND format

- create, delete and list hosted zones

- create, delete and update individual records

- create AWS extensions: failover, geolocation, latency, weighted and ALIAS records

- create, delete and use reusable delegation sets

## Installation

Installation is easy, just download the binary from the github releases page (builds are available for Linux, Mac and Windows):
https://github.com/barnybug/cli53/releases/latest

    $ sudo mv cli53-my-platform /usr/local/bin/cli53
    $ sudo chmod +x /usr/local/bin/cli53

Alternatively, on Mac you can install it using homebrew

    $ brew install cli53

To configure your Amazon credentials, either place them in a file `~/.aws/credentials`:

	[default]
	aws_access_key_id = AKID1234567890
	aws_secret_access_key = MY-SECRET-KEY

Or set the environment variables: `AWS_ACCESS_KEY_ID` and `AWS_SECRET_ACCESS_KEY`.

You can switch between different sets in the credentials file by passing
`--profile` to any command, or setting the environment variable `AWS_PROFILE`.
 For example:

        cli53 list --profile my_profile

You can also assume a specific role by passing `--role-arn` to any command.
For example:

        cli53 list --role-arn arn:aws:iam::123456789012:role/myRole

You can combine role with profile.
For example:

        cli53 list --profile my_profile --role-arn arn:aws:iam::123456789012:role/myRole

For more information, see: http://blogs.aws.amazon.com/security/post/Tx3D6U6WSFGOK2H/A-New-and-Standardized-Way-to-Manage-Credentials-in-the-AWS-SDKs

Note: for Alpine on Docker, the pre-built binaries do not work, so either use Debian, or follow the instructions below for Building from source.

## Building from source

To build yourself from source (you will need golang >= 1.5 installed):

    $ export GO15VENDOREXPERIMENT=1
    $ go get github.com/barnybug/cli53/cmd/cli53

This will produce a binary `cli53` in `$GOPATH/bin`, after this follow the steps as above.

## Getting Started

Create a hosted zone:

	$ cli53 create example.com --comment 'my first zone'

Check what we've done:

	$ cli53 list

List also supports other output formats (eg. json for scripting using [jq](https://stedolan.github.io/jq/)):

	$ cli53 list -format json | jq .[].Name

Import a BIND zone file:

	$ cli53 import --file zonefile.txt example.com

Replace with an imported zone, waiting for completion:

	$ cli53 import --file zonefile.txt --replace --wait example.com

Also you can 'dry-run' import, to check what will happen:

	$ cli53 import --file zonefile.txt --replace --wait --dry-run example.com

Create an A record pointed to 192.168.0.1 with TTL of 60 seconds:

	$ cli53 rrcreate example.com 'www 60 A 192.168.0.1'

Update this A record to point to 192.168.0.2:

	$ cli53 rrcreate --replace example.com 'www 60 A 192.168.0.2'

Delete the A record:

	$ cli53 rrdelete example.com www A

Create an MX record:

	$ cli53 rrcreate example.com '@ MX 10 mail1.' '@ MX 20 mail2.'

Create a round robin A record:

	$ cli53 rrcreate example.com '@ A 127.0.0.1' '@ A 127.0.0.2'

For CNAME records, relative domains have no trailing dot, but absolute domains should:

	$ cli53 rrcreate example.com 'login CNAME www'
	$ cli53 rrcreate example.com 'mail CNAME ghs.googlehosted.com.'

Export as a BIND zone file (for backup!):

	$ cli53 export example.com

Export fully-qualified domain names (instead of just prefixes) to `stdout`, and send AWS debug logging to `stderr`:

    $ cli53 export --full --debug example.com > example.com.txt 2> example.com.err.log

Create some weighted records:

	$ cli53 rrcreate --identifier server1 --weight 10 example.com 'www A 192.168.0.1'
	$ cli53 rrcreate --identifier server2 --weight 20 example.com 'www A 192.168.0.2'

Create an alias to an ELB:

	$ cli53 rrcreate example.com 'www AWS ALIAS A dns-name.elb.amazonaws.com. ABCDEFABCDE false'

Create an alias to an A record:

	$ cli53 rrcreate example.com 'www AWS ALIAS A server1 $self false'

Create an alias to a CNAME:

	$ cli53 rrcreate example.com 'docs AWS ALIAS CNAME mail $self false'

Create some geolocation records:

	$ cli53 rrcreate -i Africa --continent-code AF example.com 'geo 300 IN A 127.0.0.1'
	$ cli53 rrcreate -i California --country-code US --subdivision-code CA example.com 'geo 300 IN A 127.0.0.2'

Create a primary/secondary pair of health checked records:

	$ cli53 rrcreate -i Primary --failover PRIMARY --health-check 2e668584-4352-4890-8ffe-6d3644702a1b example.com 'ha 300 IN A 127.0.0.1'
	$ cli53 rrcreate -i Secondary --failover SECONDARY example.com 'ha 300 IN A 127.0.0.2'

Create a multivalue record with health checks:

	$ cli53 rrcreate -i One --multivalue --health-check 2e668584-4352-4890-8ffe-6d3644702a1b example.com 'ha 300 IN A 127.0.0.1'
	$ cli53 rrcreate -i Two --multivalue --health-check 7c90445d-ad67-47bd-9649-3ca0985e1f88 example.com 'ha 300 IN A 127.0.0.2'

Create, list and then delete a reusable delegation set:

	$ cli53 dscreate
	$ cli53 dslist
	$ cli53 dsdelete NA24DEGBDGB32

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

Please note I'll no longer be supporting this any more, so any bug reports
will be flatly closed!

## Broken CNAME exports (GoDaddy)

Some DNS providers export broken bind files, without the trailing '.'
on CNAME records. This is a requirement for absolute records
(i.e. ones outside of the qualifying domain).

If you see CNAME records being imported to route53 with an extra
mydomain.com on the end (e.g. ghs.google.com.mydomain.com), then you
need to fix your zone file before importing:

	$ perl -pe 's/((CNAME|MX\s+\d+)\s+[-a-zA-Z0-9._]+)(?!.)$/$1./i' broken.txt > fixed.txt

## Private/public zones

To manage zones that have both a private and a public zone, you must specify the
zone ID instead the domain name, which is ambiguous. This is the 13 character ID
after '/hostedzone/' you can see in the output to 'cli53 list'. eg:

    $ cli53 rrcreate ZZZZZZZZZZZZZ 'name A 127.0.0.1'

## Setting Endpoint URL

Similar to the AWS CLI, the Route 53 endpoint can be set with the --endpoint-url flag. It can be a hostname or a fully qualified URL. This is particularly useful for testing.

    $ cli53 list --endpoint-url "http://localhost:4580"

## Caveats

As Amazon limits operations to a maximum of 100 changes, if you
perform a large operation that changes over 100 resource records it
will be split. An operation that involves deletes, followed by updates
such as an import with --replace will very briefly leave the domain
inconsistent. You have been warned!

## Changelog

See: [CHANGELOG](CHANGELOG.md)
