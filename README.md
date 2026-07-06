# Apple Music ALAC / Dolby Atmos Downloader

[English](./README.md) | [简体中文](./README-CN.md)

> **Original script by Sorrow.** Modified with fixes and improvements.

---

## ⚠️ Prerequisites

**Must be installed first:**

- **[MP4Box](https://gpac.io/downloads/gpac-nightly-builds/)** - Ensure it's correctly added to your environment variables
- **[wrapper](https://github.com/WorldObservationLog/wrapper)** - Decryption program must be running before use

**Optional (for MV download):**

- **[mp4decrypt](https://www.bento4.com/downloads/)**

---

## ✨ Features

1. **Inline Covers & LRC Lyrics** - Requires `media-user-token` (see instructions below)
2. **Word-by-word & Out-of-sync Lyrics** support
3. **Artist Album Download** - Automatically download all albums from an artist
   ```bash
   go run . https://music.apple.com/us/artist/taylor-swift/159260351 --all-album
   ```
4. **Stream Decryption** - Uses Sendy McSenderson's code for download-and-decrypt streaming, solving memory issues with large files
5. **MV Download** - Requires mp4decrypt installation
6. **Interactive Search** - Arrow-key navigation for search results
   ```bash
   go run . --search [song/album/artist] "search_term"
   ```
7. **Durable Download Database** - Completed and failed items are recorded in SQLite so repeat runs can skip known downloads and repair failures
8. **Safer Output Writes** - Media, covers, lyrics, playlists, conversions, and MV mux output are written through temporary files before replacing final files
9. **Batch & Streamrip-style Workflows** - Use URL files, direct Apple IDs, dry runs, JSON summaries, doctor checks, and DB inspection commands
10. **Controlled Parallel Downloads** - `download-concurrency` now runs top-level URL jobs in parallel while keeping interactive `--select` mode serial

---

## 🎵 Supported Audio Formats

| Format | Description | Requires Subscription |
|--------|-------------|----------------------|
| `alac` | audio-alac-stereo | ✅ |
| `ec3` | audio-atmos / audio-ec3 | ✅ |
| `aac` | audio-stereo | ✅ |
| `aac-lc` | audio-stereo | ✅ |
| `aac-binaural` | audio-stereo-binaural | ✅ |
| `aac-downmix` | audio-stereo-downmix | ✅ |
| `MV` | Music Video | ✅ |

> **Note:** For `aac-lc`, `MV`, and `lyrics`, you must provide a valid `media-user-token` from an active subscription.

---

## 🚀 Usage

### Native macOS (Apple Silicon / arm64)

1. Install native arm64 dependencies with Homebrew:

   ```bash
   brew install go ffmpeg gpac bento4
   ```

   `gpac` provides `MP4Box`; `bento4` provides `mp4decrypt` for MV/station downloads.

2. Build a native macOS arm64 package:

   ```bash
   make package-macos-arm64
   ```

   The packaged binary and default config are written to `dist/darwin-arm64/`, with an archive at `dist/apple-music-dl-darwin-arm64.tar.gz`.

3. Run locally:

   ```bash
   cd dist/darwin-arm64
   ./apple-music-dl --help
   ./apple-music-dl https://music.apple.com/us/album/whenever-you-need-somebody-2022-remaster/1624945511
   ```

   The app automatically checks common Apple Silicon Homebrew paths such as `/opt/homebrew/bin`. If your tools live elsewhere, set these in `config.yaml`:

   ```yaml
   ffmpeg-path: "/opt/homebrew/bin/ffmpeg"
   mp4box-path: "/opt/homebrew/bin/MP4Box"
   mp4decrypt-path: "/opt/homebrew/bin/mp4decrypt"
   ```

### Running with Docker

1. Ensure the [wrapper](https://github.com/WorldObservationLog/wrapper) decryption program is running

2. Start the downloader:

```bash
# Show help
docker run --network host -v ./downloads:/downloads ghcr.io/zhaarey/apple-music-downloader --help

# Download albums
docker run --network host -v ./downloads:/downloads ghcr.io/zhaarey/apple-music-downloader https://music.apple.com/ru/album/children-of-forever/1443732441

# Download single song
docker run --network host -v ./downloads:/downloads ghcr.io/zhaarey/apple-music-downloader --song https://music.apple.com/ru/album/bass-folk-song/1443732441?i=1443732453

# Interactive selection
docker run -it --network host -v ./downloads:/downloads ghcr.io/zhaarey/apple-music-downloader --select https://music.apple.com/ru/album/children-of-forever/1443732441

# Music video selection / quality picker
docker run -it --network host -v ./downloads:/downloads ghcr.io/zhaarey/apple-music-downloader --select https://music.apple.com/us/artist/taylor-swift/159260351

# Download playlists
docker run --network host -v ./downloads:/downloads ghcr.io/zhaarey/apple-music-downloader https://music.apple.com/us/playlist/taylor-swift-essentials/pl.3950454ced8c45a3b0cc693c2a7db97b

# Dolby Atmos
docker run --network host -v ./downloads:/downloads ghcr.io/zhaarey/apple-music-downloader --atmos https://music.apple.com/us/album/1989-taylors-version-deluxe/1713845538

# AAC format
docker run --network host -v ./downloads:/downloads ghcr.io/zhaarey/apple-music-downloader --aac https://music.apple.com/us/album/1989-taylors-version-deluxe/1713845538

# Debug/View quality
docker run --network host -v ./downloads:/downloads ghcr.io/zhaarey/apple-music-downloader --debug https://music.apple.com/ru/album/miles-smiles/209407331
```

**Custom Configuration:**

Mount your own `config.yaml`:

```bash
docker run --network host -v ./downloads:/downloads -v ./config.yaml:/app/config.yaml ghcr.io/zhaarey/apple-music-downloader [args]
```

> **Note:** Ensure `config.yaml` exists in your current directory before running. If it doesn't exist, Docker will create an empty directory instead of a file, causing the container to fail.

---

### Running Locally (Go)

If you installed the packaged wrapper command, replace `go run .` with `amdl` in the examples below.

1. Ensure the [wrapper](https://github.com/WorldObservationLog/wrapper) decryption program is running

2. **Download albums:**
   ```bash
   go run . https://music.apple.com/us/album/whenever-you-need-somebody-2022-remaster/1624945511
   ```

3. **Download single song:**
   ```bash
   go run . --song https://music.apple.com/us/album/never-gonna-give-you-up-2022-remaster/1624945511?i=1624945512
   # or
   go run . https://music.apple.com/us/song/you-move-me-2022-remaster/1624945520
   ```

4. **Interactive selection:**
   ```bash
   go run . --select https://music.apple.com/us/album/whenever-you-need-somebody-2022-remaster/1624945511
   ```
   For album and playlist URLs, select tracks/items. For artist URLs, select albums and music videos in one table. For direct music video URLs, select video quality.

5. **Artist / music video selection:**
   ```bash
   go run . --select https://music.apple.com/us/artist/taylor-swift/159260351
   # or
   go run . --select https://music.apple.com/us/music-video/example/123456789
   ```

6. **Download playlists:**
   ```bash
   go run . https://music.apple.com/us/playlist/taylor-swift-essentials/pl.3950454ced8c45a3b0cc693c2a7db97b
   # or
   go run . https://music.apple.com/us/playlist/hi-res-lossless-24-bit-192khz/pl.u-MDAWvpjt38370N
   ```

7. **Dolby Atmos:**
   ```bash
   go run . --atmos https://music.apple.com/us/album/1989-taylors-version-deluxe/1713845538
   ```

8. **AAC format:**
   ```bash
   go run . --aac https://music.apple.com/us/album/1989-taylors-version-deluxe/1713845538
   ```

9. **View quality info:**
   ```bash
   go run . --debug https://music.apple.com/us/album/1989-taylors-version-deluxe/1713845538
   ```

10. **Use a specific config file:**
    ```bash
    go run . --config config.yaml https://music.apple.com/us/album/whenever-you-need-somebody-2022-remaster/1624945511
    ```

11. **Validate setup before downloading:**
    ```bash
    go run . --config config.yaml --config-validate
    go run . --config config.yaml --doctor
    go run . --config config.yaml --config-upgrade
    ```

12. **Preview without writing media files:**
    ```bash
    go run . --config config.yaml --dry-run --json-summary https://music.apple.com/us/album/whenever-you-need-somebody-2022-remaster/1624945511
    ```

13. **Download from a batch file:**
    ```bash
    # urls.txt can contain one URL per line
    go run . --config config.yaml --file urls.txt --json-summary
    ```

14. **Download by Apple Music ID:**
    ```bash
    go run . --config config.yaml --id-type album --id 1624945511 --storefront us
    go run . --config config.yaml --id-type song --id 1624945520 --storefront us --json-summary
    ```

📖 [Chinese Tutorial (Method 3)](https://telegra.ph/Apple-Music-Alac%E9%AB%98%E8%A7%A3%E6%9E%90%E5%BA%A6%E6%97%A0%E6%8D%9F%E9%9F%B3%E4%B9%90%E4%B8%8B%E8%BD%BD%E6%95%99%E7%A8%8B-04-02-2)

---

## 🧰 Reliability & Workflow Commands

### Database-backed skips and repair

By default, this fork creates `amdl.db` next to the active config/runtime directory. Downloads are recorded only after post-processing succeeds. Failed items are stored separately and can be retried later.

```bash
# List completed records
go run . --config config.yaml --db-list downloads

# List failed records
go run . --config config.yaml --db-list failed

# Retry only failed records
go run . --config config.yaml --repair --json-summary

# Disable database use for one run
go run . --config config.yaml --no-db "https://music.apple.com/us/album/example/123"
```

If a track already exists locally, a repeat run records it as `file-exists` and backfills the database where possible. This makes old downloads visible to future skip checks.

SQLite is configured with WAL mode and a busy timeout. Parallel workers also use in-memory per-item locks so duplicate URLs in the same batch do not download the same track or music video twice.

### Config upgrade helper

Use this when your existing `config.yaml` predates the newer safety/workflow keys:

```bash
go run . --config config.yaml --config-upgrade
```

The command does not rewrite your config. It prints missing managed keys and a YAML snippet you can add manually. `--doctor` also warns when managed keys are missing.

### JSON summary

Use `--json-summary` for automation-friendly output:

```bash
go run . --config config.yaml --json-summary --file urls.txt
```

The summary contains `started_at`, `completed_at`, `counts`, `downloaded`, `skipped`, and `failed`. Each item includes storefront, media type, Apple ID, codec, quality, path, reason, and error when available.

The older `--json` flag is still kept for compatibility and prints the added-tracks array.

### Parallel downloads

`download-concurrency` controls how many top-level URLs are processed at once:

```yaml
download-concurrency: 3
```

Then run a batch:

```bash
go run . --config config.yaml --file urls.txt --json-summary
```

Notes:

- Default remains `1` for compatibility.
- `--select` is interactive and automatically uses concurrency `1`.
- Queue-level logs are buffered per top-level URL so parallel batch output stays easier to read.
- MV segments use a separate `mv-segment-concurrency` setting.
- Keep `requests-per-minute`, `retry-count`, and `request-timeout-seconds` conservative if you raise concurrency.

### Search workflows

```bash
# Download the first search result
go run . --config config.yaml --search song "artist title" --search-first

# Save search results without downloading
go run . --config config.yaml --search album "album title" --search-output results.json
```

---

## ⚙️ Important Config Keys

Newer safety and workflow defaults are included in `config.yaml.example`:

```yaml
database-enabled: true
database-path: "amdl.db"
failed-downloads-enabled: true

download-concurrency: 1
mv-segment-concurrency: 6
requests-per-minute: 60
retry-count: 3
retry-backoff-ms: 500
request-timeout-seconds: 30

allow-nonlocal-wrapper: false
quality-fallback: "skip"      # skip | best-available

artist-filter-singles-eps: false
artist-filter-live: false
artist-filter-compilations: false
artist-filter-duplicates: false
```

Wrapper ports are loopback-only by default. If you intentionally expose wrapper ports beyond localhost, set `allow-nonlocal-wrapper: true`, but do this only on a trusted network.

---

## 📝 Getting media-user-token (For Lyrics)

1. Open [Apple Music](https://music.apple.com) and log in
2. Open Developer Tools (F12)
3. Navigate to `Application → Storage → Cookies → https://music.apple.com`
4. Find the cookie named `media-user-token` and copy its value
5. Paste the value into `config.yaml` under the `media-user-token` setting
6. Save the file and start the script

---

## 🌐 Getting Translation & Pronunciation Lyrics (Beta)

> **Note:** These features are currently in beta.

1. Open [Apple Music Beta](https://beta.music.apple.com) and log in
2. Open Developer Tools (F12) and switch to the **Network** tab
3. Search for a song that supports translation/pronunciation lyrics (K-Pop songs recommended)
4. Press **Ctrl+R** to refresh and let DevTools capture network traffic
5. Play the song and click the lyrics button - look for a request named `syllable-lyrics`
6. Stop recording (click the red circle button), then select the **Fetch/XHR** tab
7. Click on the `syllable-lyrics` request to view details
8. Find the URL containing: `.../syllable-lyrics?l=<language_code>&extend=ttmlLocalizations`
9. Copy the language value and paste it into `config.yaml`
10. **Optional:** To disable pronunciation, remove the corresponding value in config.yaml: `...%5D=<remove_this_value>&extend...`
11. Save and run the script as usual

---

## 👏 Special Thanks

- **chocomint** - Created `agent-arm64.js`

---
