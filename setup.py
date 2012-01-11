#!/usr/bin/python2.6
"""Setup file for cli53."""

__author__ = 'contact@martincozzi.com'

from setuptools import setup

setup(
    name='cli53',
    version='0.1',
    description='Command line script to administer the Amazon Route 53 dns service.',
    package_dir={'': 'src'},
    packages=['cli53'],
    install_requires=[
        'boto',
        'dnspython',
        'elementtree',
        'uuid',
        ],
    entry_points={
        'console_scripts': [
            'cli53 = cli53.cli53:main',
            ],
        },
    zip_safe=False,
    )
