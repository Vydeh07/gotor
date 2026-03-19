# gotor 

A BitTorrent client built from scratch in Go — no BitTorrent libraries used.

## What it does

Downloads files from the BitTorrent network by implementing the full protocol from scratch:
- Decodes `.torrent` files (custom bencode parser)
- Contacts trackers to discover peers
- Connects to peers over TCP and performs the BitTorrent handshake
- Downloads file pieces concurrently from multiple peers
- Verifies every piece using SHA-1 before writing to disk

## How it works
```
.torrent file → bencode decode → tracker HTTP request → peer list
→ TCP handshake × N peers → concurrent piece download → SHA-1 verify → file
```

## Usage
```bash
# clone the repo
git clone https://github.com/Vydeh07/gotor.git
cd gotor

# drop any .torrent file in the folder
go run main.go
```

## Architecture
```
gotor/
├── bencode/     # custom bencode decoder
├── torrent/     # .torrent file parser + SHA-1 info hash
├── tracker/     # HTTP tracker communication, peer discovery  
├── peer/        # TCP connections, handshake, message protocol
└── downloader/  # concurrent piece downloader, verification, disk I/O
```

## Implementation details

- **Bencode parser** — hand-written recursive descent parser for the bencode format used by all torrent files
- **Info hash** — SHA-1 of the raw bencode bytes of the info dict, used to identify the torrent to trackers and peers
- **Tracker protocol** — HTTP GET with URL-encoded binary params, parses compact peer response (6 bytes per peer)
- **Handshake** — 68 byte message: protocol string + reserved bytes + info hash + peer ID
- **Message protocol** — length-prefixed binary messages with type IDs (choke, unchoke, interested, bitfield, request, piece)
- **Block pipelining** — keeps 5 requests in flight simultaneously per peer for maximum throughput
- **Concurrent downloads** — one goroutine per peer, all pulling from a shared work queue channel
- **Piece verification** — every 256KB piece is SHA-1 hashed and compared against the torrent manifest before being written to disk
- **Disk streaming** — pieces written directly to final position in output file as they arrive, no full-file RAM buffer

## Built with

- Go standard library only — no external dependencies
- `crypto/sha1`, `net`, `encoding/binary`, `sync`
