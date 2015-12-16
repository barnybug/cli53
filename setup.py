from setuptools import setup, find_packages
from setuptools import Command

__version__ = '0.5.2'
long_description = file('README.markdown', 'r').read()

class tag(Command):
    """Tag git release."""

    description = __doc__
    user_options = []

    def initialize_options(self):
        pass

    def finalize_options(self):
        pass

    def run(self):
        import subprocess
        ret = subprocess.call(['git', 'tag', '-a', __version__, '-m', __version__])
        if ret:
            raise SystemExit("git tag failed")
        ret = subprocess.call(['git', 'push', '--tags'])
        if ret:
            raise SystemExit("git push --tags failed")

setup(
    name='cli53',
    version=__version__,
    description='Command line script to administer the Amazon Route 53 DNS service',
    long_description=long_description,
    license='MIT',
    author='Barnaby Gray',
    author_email='barnaby@pickle.me.uk',
    url='http://loads.pickle.me.uk/cli53/',
    install_requires=['boto', 'argparse', 'dnspython'],
    scripts=['scripts/cli53'],
    packages=find_packages(),
    classifiers=[
        "Development Status :: 3 - Alpha",
        "Topic :: Utilities",
        "License :: OSI Approved :: MIT License",
    ],
    cmdclass={
        'tag': tag,
    },
)
