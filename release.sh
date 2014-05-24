#!/bin/bash -e

subl -w setup.py:4
VERSION=$(python setup.py --version)

# make changelog
echo -e "$VERSION\n" > /tmp/changelog
git log --format='- %s%n' $(git describe --abbrev=0).. >> /tmp/changelog
subl -w README.markdown:166 /tmp/changelog
rm /tmp/changelog

git add README.markdown setup.py
git commit -m $VERSION
git push
python setup.py release

echo "$VERSION released"
