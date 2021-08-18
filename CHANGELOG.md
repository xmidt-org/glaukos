# Changelog
All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](http://keepachangelog.com/en/1.0.0/)
and this project adheres to [Semantic Versioning](http://semver.org/spec/v2.0.0.html).


## [Unreleased]

## [v0.3.0]

### Added
- Add time validation struct and interface. [#44](https://github.com/xmidt-org/glaukos/pull/45)
- Add validator interface to validate events. [#48](https://github.com/xmidt-org/glaukos/pull/48)
- Add event finder to separate out searching through history of events from parsers. [#49](https://github.com/xmidt-org/glaukos/pull/49)
- Add `TimeElapsedParser`. [#55](https://github.com/xmidt-org/glaukos/pull/55)
- Add rate limiter and metrics for codex client. [#57](https://github.com/xmidt-org/glaukos/pull/57)
- Add metrics to track event types and how long events are in memory. [#59](https://github.com/xmidt-org/glaukos/pull/59)
- Switch to `touchstone` library. [#62](https://github.com/xmidt-org/glaukos/pull/62)
- Remove themis libraries and switch to `zap`, `sallust`, `httpaux`, and `arrangehttp` libraries. [#64](https://github.com/xmidt-org/glaukos/pull/64)

### Changed
- Change `TimeElapsedParser` to `RebootDurationParser`, reworking implementation logic. [#67](https://github.com/xmidt-org/glaukos/pull/67)
- Rename packages and move files. [#46](https://github.com/xmidt-org/glaukos/pull/46)
- Refactor to only use Events, remove use of wrp.Message beyond converting a wrp.Message to Event when a message is received through the webhook. [#47](https://github.com/xmidt-org/glaukos/pull/47)
- Refactor to use `interpreter` library. [#53](https://github.com/xmidt-org/glaukos/pull/53)
- Modify `Parser` interface. [#55](https://github.com/xmidt-org/glaukos/pull/55)
- Use `httpaux/retry` instead of `webpa-common/xhttp` library. [#57](https://github.com/xmidt-org/glaukos/pull/57)

### Removed
- Create `bootTimeParser` and `totalBootTimeParser` through configuration of time-elapsed parser and remove code files that implement these parsers. [#55](https://github.com/xmidt-org/glaukos/pull/55)

## [v0.2.4]
- Add more detailed logging for long durations. [#43](https://github.com/xmidt-org/glaukos/pull/43)

## [v0.2.3]
- Fix for metadata keys that don't contain a `/`. [#42](https://github.com/xmidt-org/glaukos/pull/42)

## [v0.2.2]
- Allow for `/boot-time` and `boot-time` as metadata keys when getting the boot-time from a wrp message or event. [#41](https://github.com/xmidt-org/glaukos/pull/41)

## [v0.2.1]
- Add more detailed logging. [#40](https://github.com/xmidt-org/glaukos/pull/40)
  
## [v0.2.0]
- Change boot-time calculation to use the birthdate of the request, remove absolute value in boot-time calculation. [#31](https://github.com/xmidt-org/glaukos/pull/31)
- Add a parser to calculate the time between reboot-pending and fully-manageable events. [#35](https://github.com/xmidt-org/glaukos/pull/35)

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

[Unreleased]: https://github.com/xmidt-org/glaukos/compare/v0.3.0..HEAD
[v0.3.0]: https://github.com/xmidt-org/glaukos/compare/v0.2.4..v0.3.0
[v0.2.4]: https://github.com/xmidt-org/glaukos/compare/v0.2.3..v0.2.4
[v0.2.3]: https://github.com/xmidt-org/glaukos/compare/v0.2.2..v0.2.3
[v0.2.2]: https://github.com/xmidt-org/glaukos/compare/v0.2.1..v0.2.2
[v0.2.1]: https://github.com/xmidt-org/glaukos/compare/v0.2.0..v0.2.1
[v0.2.0]: https://github.com/xmidt-org/glaukos/compare/v0.1.1..v0.2.0
[v0.1.1]: https://github.com/xmidt-org/glaukos/compare/v0.1.0..v0.1.1
[v0.1.0]: https://github.com/xmidt-org/glaukos/compare/v0.0.1..v0.1.0
[v0.0.1]: https://github.com/xmidt-org/glaukos/compare/0.0.1...v0.0.1
