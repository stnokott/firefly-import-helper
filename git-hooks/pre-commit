#!/bin/sh

touch test.txt

VERSION=$(git describe --tags | awk "{split(\$0,a,\"-\"); print a[1];}")

echo -e "package util\n\nconst Version = \"$VERSION\"" > internal/util/version.go

git add internal/util/version.go
git diff-index --quiet HEAD || git commit -m "change version number to $VERSION"

exit 0