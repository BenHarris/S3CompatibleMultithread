# lw-ObjStor4cPanel

[![Build Status](https://travis-ci.org/jakdept/lw-ObjStor4cPanel.svg?branch=master)](https://travis-ci.org/jakdept/lw-ObjStor4cPanel)

**_This is experimental for the time being_**

This is a custom transport script that will allow cPanel/WHM backups to be sent to Liquid Web Object Storage.

I assume the user understands the concept of access keys, secret keys, buckets, and has a bucket already set up in Object Storage.

## Requirements ##

Only docker is required to build the RPMs for this - see instructions below.

Before installing, you must have a valid Access Key and Secret Key in Object Storage, and a valid bucket within that account.

## Usage ##

> You must already have a bucket created in Object Storage.

> You must already have a valid Access Key and Secret Key ([manage.liquidweb.com](https://manage.liquidweb.com))
1. Go clone this repository container - `https://github.com/jakdept/rpmbuild-docker.git`
1. Build the github CentOS6 and CentOS7 docker containers.
1. To build the Cent6 RPM run the following:
`docker run --rm -v $(pwd)/:/home/rpmbuild/package jakdept/rpmbuild:cent6 rpmbuild -bb SPECS/S3CompatibleMultithread.spec`
1. To build the Cent7 RPM run the following:
`docker run --rm -v $(pwd)/:/home/rpmbuild/package jakdept/rpmbuild:cent7 rpmbuild -bb SPECS/S3CompatibleMultithread.spec`
1. Get your build binary from the folder `x86_64` and put it on the server.
1. Install that RPM on the server.
1. Go to **Backup Configuration** in WHM and edit the **LW Object Storage** destination.
1. Change **Remote Host** to the *name of the bucket* you are going to use.
1. Change **Remote Account Username** to the *Access Key*.
1. Change **Remote Password** to the *Secret Key*.
1. *Backup Directory* can be modified if you want to put backups in a different directory on the server. If backing multiple servers up to the same bucket, you should use different *Backup Directories*.

## Testing ##

You can run some (limited) tests without a valid configuration for Liquidweb's Object Storage, however in order to run all tests you need a valid configuration. This configuration should be stored in env vars:

```bash
export PWD="/"
export ACCESSKEY="accessKey"
export SECRETKEY="secretKey"
export BUCKET="bucketName"
```

With this in place, and go installed, you can view test coverage with:

```
go test -v -coverprofile=$TMPDIR/c.out && go tool cover -html=$TMPDIR/c.out
```