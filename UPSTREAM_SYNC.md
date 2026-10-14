# Upstream synchronization audit

Checked on 2026-10-14 against Zhaarey's live `main` at
[`47413807afab6ce22e671d009cb19715290891de`](https://github.com/zhaarey/apple-music-downloader/commit/47413807afab6ce22e671d009cb19715290891de).

The previous shared commit was `9dd6dede39c8b5ea42d5a741fee903003ee72690`.
The fork had 11 unique commits; upstream had 72 unseen commits, including merge commits.
This integration is a two-parent merge, retaining upstream ancestry so the next sync
can use Git's normal merge base.

## Changes examined and integrated

| Area | Upstream changes | Integration |
| --- | --- | --- |
| Audio selection and stations | Default ALAC limits, real station-master variant resolution, playlist writing | Retained fork selectors; zero ALAC limit selects up to 192 kHz; stations resolve real variants; fork playlist writer retained |
| Network and wrapper | HTTP/SOCKS proxies, wrapper-lite HTTP endpoints, prefetched templates, first-response CDN download fixes | Shared `internal/download`, `internal/wrapper`, and FairPlay engine; fork catalog retry/rate controls and separate artwork deadlines retained |
| FairPlay performance | Parallel template decryption, pure-Go Temari port, bounded stream/memory handling | Current `internal/fairplay-rip` and `internal/temari`; old runv2/runv3 and Frida agents removed |
| MV and audio containers | Pure-Go defragmentation, muxing and tagging; CMAF fixes; sample offsets above 2/4 GiB | Shared media implementations; fork stages files with unique basenames through processing before publishing final media |
| Video DRM | In-process Widevine, PlayReady SL3000 selection, caption-track preservation, bounded segmented downloads | Current upstream engines; fork's richer interactive picker propagates DRM metadata; configurable segment workers retained |
| Lyrics and API | Translation/transliteration TTML, wrapper lyrics-not-found responses, fixed artist endpoint, pagination | Current lyrics/wrapper code and API domains; fork empty-response guards, error context and bounded catalog clients retained |
| Config and CLI | Hierarchical koanf configuration, embedded defaults, standalone config creation, portable program names | Current upstream packages plus flat/nested compatibility adapter; maintenance commands available in the fork CLI |
| Updates and releases | Version metadata, checksum-verified updates, config migration, VERSION-driven releases, Android DNS/cgo fix | Upstream version/updater/release code; release-built binaries follow their building repository; source default points to yttxz |
| Packaging | Upstream removed Docker and switched release artifacts | Fork Docker and native macOS Makefile retained and adapted to the new source tree and embedded assets; CI adds race checks |

## Intentional fork differences

The entry point still runs the fork's database-aware, concurrent workflow runner.
Shared upstream media helpers are exposed through a narrow `internal/app/bridge.go`;
upstream runner sources remain available for future merges. No duplicate legacy
DRM engines are kept. `utils/structs` holds the flat compatibility schema and
`utils/httpx` the fork's retry transport.

SQLite downloads/failures, repairs, batch input, direct IDs, dry runs, richer search
and selection, artist filters, JSON summaries, in-flight deduplication, atomic
writes, artwork progress, and independent artwork timeouts are retained.

Legacy flat configuration is read without rewriting it. Nested values override
flat aliases. Config migration seeds missing nested options from legacy values,
so inserting new keys cannot reset credentials, folders, or explicit boolean/zero
settings. The old TCP wrapper settings are accepted for compatibility but downloads
require the wrapper-lite HTTP endpoint. Nonlocal wrapper endpoints require the
existing `allow-nonlocal-wrapper` opt-in.

The upstream MP4 tagging library uses a global basename-plus-millisecond temp path.
The first parallel test run exposed collisions between packages using `mv.mp4`.
Both fixture builders now generate distinct basenames; production fork media is
also staged under unique basenames before tagging. The same fixture/tag tests pass
when run separately and in the complete parallel suite.

## Verification

The complete upstream and fork tests, race detector, vet, native packaging and
cross-platform build checks are recorded in the integration PR. Local CLI smoke
checks cover help/version, both config formats, config creation/migration and DB
inspection. Fixture tests cover media processing, sample preservation, tag writing,
large offsets, bounded segment ordering, wrapper contracts and lyric conversion.
A live subscriber download is not asserted: it requires a working authenticated
wrapper-lite service. Verified locally:

- `go test -count=1 ./...` and `go test -race -count=1 ./...` passed.
- `go vet ./...` passed; native macOS arm64 packaging succeeded.
- Linux amd64 and Windows amd64 cross-builds succeeded.
- Repeated media fixture/tagging suites passed for 20 iterations together.
- CLI smoke checks passed for side-effect-free help/version, default config creation,
  flat/nested config validation, backed-up migration, DB inspection, dry-run JSON,
  and invalid/missing custom config error exits.
- Docker image built on Linux arm64 with Go 1.25.5; container config validation
  and version output passed with network access disabled.

These checks do not establish live subscriber playback. The host Go version is
1.27.2; the container build additionally verifies compilation on Go 1.25.5. CI
uses the upstream minimum Go 1.23.1 on Linux, Windows, and macOS and gates release
builds on the complete test/race/vet/build workflow.

## Audited upstream implementation commits

```text
a9ec9d6 Fix ALAC selection when alac-max is unset
82cf29d Fix station playlist variant resolution
4559652 fix: bypass Apple API 503 / lyrics block on cloud servers via optional proxy support
74b059c feat: 适配最新模板解密
148788c fix: 非debug模式显示过多信息
102ea9f opt: 多协程解密提升模板解密速度
5411c8b opt: 写死prefetch template，优化解密速度
d1529e6 opt: 优化大文件网络下载
9851e55 opt: 优化输出字符显示
7fccd1e merge (#3)
dc835f1 opt: 优化解密性能，解密速度x2
1df3c21 update markdown
63dc6dc Update Wrapper-SETUP.md
db25726 update readme
0d895e0 add auto get music-token
f53fc17 feat(runv4): support custom key-server paths and enveloped responses
2bcfcc1 fix(runv4): prevent 4MB CDN download cutoff and eliminate retry delay
7d1430e wrapper-lite support
b10ddc8 fix(runv4): stop downloads stalling at 4 MiB by consuming the first response
32b1c7a Update config.yaml.example
9aaf0f9 change to wrapper-lite
0e2559c Add internal directory to Docker ignore exceptions
34c3dcd Update Go version and modify workflow steps
bb3b473 Fixed the artist's incorrect ampapi address.
68c231a Refactor lyrics tests for updated TTML structure
6d93938 Refactor lyrics.go to improve translation handling
6785417 Use DefragmentMP4 and embed covers via MP4 tags
64c07b7 Replace MV remux with pure Go muxer
facad1d Replace MP4Box station tagging with pure Go
0f0d6f1 Fix binary MP4 fixture checkout
ba92296 Add PlayReady MV decryption and preserve caption tracks
6410006 Optimize alac-fix runtime output and add unit tests
54a1a8b Refactor wrapper-lite interactions into internal/wrapper
8980de1 Fix MediaInfo CMAF conformance error on progressive M4A audio
48d005e Refactor configuration system with koanf and hierarchical YAML structure
2023ace Fix save-m3u8-playlist flag not writing playlist files
83a6905 feat: implement self-update system and refactor CI/CD workflows
a153f6f ci: skip macos test in ci and add android arm64 release target
6b23152 fix: auto-create config.yaml from embedded template in standalone mode
5e8239e feat: dynamic CLI usage program name and clean download path defaults
72b6c50 feat: reorganize path templates and tag configurations for v0.0.4
ffb61ae fix: handle cross-platform path separators in getProgName
3a6b1b5 fix: gracefully handle 404 response from wrapper-lite lyrics endpoint
a45c6d7 docs: reorganize installation into quick start and add developer guide
6306608 refactor: restructure widevine-rip with descriptive names and pluggable license backends
456d87a feat: replace Temari cdylib with a pure-Go port
399aa14 perf: speed up per-track processing around downloads
221545f perf: bound memory while decrypting and downloading MV streams
e9bff3b fix: keep defragmented MVs taggable above 2 GiB and 4 GiB
631e33f build: bump go-mp4tag to 472a1d1 for 64-bit and unsigned box sizes
c4ed62c perf: bound alacfix memory instead of loading the whole track
54fd1fe fix(updater): make self-update and config migration reliable
ed9e37c fix(config): prefer embedded template over config.yaml.example in cwd
487f705 fix(android): build Android release with cgo so DNS works on Termux
75bb68c build(release): drive releases from VERSION and point to upstream
```
