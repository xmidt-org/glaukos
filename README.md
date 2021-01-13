# glaukos

Glaukos is a service that produces metrics about the XMiDT cluster as a whole.

[![Build Status](https://github.com/xmidt-org/glaukos/workflows/CI/badge.svg)](https://github.com/xmidt-org/glaukos/actions)
[![codecov.io](http://codecov.io/github/xmidt-org/glaukos/coverage.svg?branch=main)](http://codecov.io/github/xmidt-org/glaukos?branch=main)
[![Go Report Card](https://goreportcard.com/badge/github.com/xmidt-org/glaukos)](https://goreportcard.com/report/github.com/xmidt-org/glaukos)
[![Apache V2 License](http://img.shields.io/badge/license-Apache%20V2-blue.svg)](https://github.com/xmidt-org/glaukos/blob/main/LICENSE)
[![Quality Gate Status](https://sonarcloud.io/api/project_badges/measure?project=xmidt-org_glaukos&metric=alert_status)](https://sonarcloud.io/dashboard?id=xmidt-org_glaukos)
[![GitHub release](https://img.shields.io/github/release/xmidt-org/glaukos.svg)](CHANGELOG.md)

## Summary

Glaukos is a service that provides metrics on the XMiDT cluster as a whole. Currently, codex provides device-specific data, but glaukos will generate prometheus metrics that will give information on the entire cluster, such as the boot-time of CPE devices and other metadata information.


## Table of Contents
  - [Code of Conduct](#code-of-conduct)
  - [Details](#details)
  - [Build](#build)
  - [Contributing](#contributing)

## Code of Conduct

This project and everyone participating in it are governed by the [XMiDT Code Of Conduct](https://xmidt.io/code_of_conduct/). 
By participating, you agree to this Code.

## Details

Glaukos parses metadata fields from incoming device-status events from caduceus and generates metrics from those. It also queries the codex database and performs calculations to generate metrics regarding the boot-time of various devices.

## Build

### Source

In order to build from source, you need a working 1.x Go environment.
Find more information on the [Go website](https://golang.org/doc/install).

Then, clone the repository and build using make:

```bash
git clone git@github.com:xmidt-org/glaukos.git
cd hecate
make build
```

### Makefile

The Makefile has the following options you may find helpful:

- `make build`: builds the glaukos binary
- `make docker`: fetches all dependencies from source and builds a glaukos docker image
- `make test`: runs unit tests with coverage for glaukos
- `make clean`: deletes previously-built binaries and object files

### Docker

The docker image can be built either with the Makefile or by running a docker
command.  Either option requires first getting the source code.

See [Makefile](#Makefile) on specifics of how to build the image that way.

If you'd like to build it without make, follow these instructions based on your use case:

- Local testing

```bash
go mod vendor
docker build -t glaukos:local -f deploy/Dockerfile .
```

This allows you to test local changes to a dependency. For example, you can build
a glaukos image with the changes to an upcoming changes to [webpa-common](https://github.com/xmidt-org/webpa-common) by using the [replace](https://golang.org/ref/mod#go) directive in your go.mod file like so:

```go.mod
replace github.com/xmidt-org/webpa-common v1.10.2 => ../webpa-common
```

**Note:** if you omit `go mod vendor`, your build will fail as the path `../webpa-common` does not exist on the builder container.

- Building a specific version

```bash
git checkout v0.5.1
docker build -t glaukos:v0.5.1 -f deploy/Dockerfile .
```

## Contributing

Refer to [CONTRIBUTING.md](CONTRIBUTING.md).
