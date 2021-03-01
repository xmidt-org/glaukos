# Changelog
All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](http://keepachangelog.com/en/1.0.0/)
and this project adheres to [Semantic Versioning](http://semver.org/spec/v2.0.0.html).

## [Unreleased]
- Change boot-time calculation to use the time that the request is received, remove absolute value in boot-time calculation. [#31](https://github.com/xmidt-org/glaukos/pull/31)

## [v0.1.1]
- Nothing has changed.

## [v0.1.0]
- Change histogram buckets to account for long boot-times. [#19](https://github.com/xmidt-org/glaukos/pull/19)
- Use hash token factory to verify secret when configured. [#18](https://github.com/xmidt-org/glaukos/pull/18)
- Add circuit breaker to prevent overloading codex when codex is already under stress. [#17](https://github.com/xmidt-org/glaukos/pull/17)
- Allow for the http client used by the CodexClient to be configurable. [#16](https://github.com/xmidt-org/glaukos/pull/16)
- Use `xlog` instead of `webpa-common/logging`. [#15](https://github.com/xmidt-org/glaukos/pull/15)
- Add unit tests [#12](https://github.com/xmidt-org/glaukos/pull/12)
- Add queue to process incoming caduceus events. [#11](https://github.com/xmidt-org/glaukos/pull/11)
- Add initial app files and working docker-compose cluster. [#7](https://github.com/xmidt-org/glaukos/pull/7)
- Add queue to process incoming events. [#11](https://github.com/xmidt-org/glaukos/pull/11)

## [v0.0.1]
- Initial creation

[Unreleased]: https://github.com/xmidt-org/glaukos/compare/v0.1.1..HEAD
[v0.1.1]: https://github.com/xmidt-org/glaukos/compare/v0.1.0..v0.1.1
[v0.1.0]: https://github.com/xmidt-org/glaukos/compare/v0.0.1..v0.1.0
[v0.0.1]: https://github.com/xmidt-org/glaukos/compare/0.0.1...v0.0.1
