from setuptools import setup

__version__ = '0.2.1'
long_description = '''
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

Further documentation is available, e.g.::

	$ cli53 --help
	$ cli53 rrcreate --help
'''

setup(name='cli53',
      version=__version__,
      description='Command line script to administer the Amazon Route 53 DNS service',
      long_description=long_description,
      license='MIT',
      author='Barnaby Gray',
      author_email='barnaby@pickle.me.uk',
      url='http://github.com/barnybug/cli53/',
      install_requires=['boto', 'argparse', 'dnspython'],
      scripts=['scripts/cli53'],
      classifiers=[
        "Development Status :: 3 - Alpha",
        "Topic :: Utilities",
        "License :: OSI Approved :: MIT License",
        ],
      )
