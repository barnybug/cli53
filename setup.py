from setuptools import setup

__version__ = '0.2.1'

setup(name='cli53',
      version=__version__,
      description='Command line script to administer the Amazon Route 53 DNS service',
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
