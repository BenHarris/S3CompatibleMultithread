## Build the build container
docker build -t rpmbuild:cent7 github-cent7

## Build CentOS 7 RPM
docker run --rm -v ${PWD}/:/home/rpmbuild/package rpmbuild:cent7 rpmbuild -bb package/S3CompatibleMultithread.spec

## Get your build binary from the folder `x86_64` and put it on the server.