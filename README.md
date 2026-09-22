<div align="center">

<br>

<img width="50%" alt="gostat logo" src="https://user-images.githubusercontent.com/77400522/204132452-9c0182e1-860f-4c79-87f9-a18c68e2de53.png">

[![version](https://img.shields.io/github/v/tag/ghdwlsgur/gostat?color=success&label=version&sort=semver)](https://github.com/ghdwlsgur/gostat/releases)
[![CI](https://github.com/ghdwlsgur/gostat/actions/workflows/ci.yml/badge.svg)](https://github.com/ghdwlsgur/gostat/actions/workflows/ci.yml)
[![license](https://img.shields.io/github/license/ghdwlsgur/gostat?color=success)](./LICENSE)

</div>

# Overview

`gostat` sends an HTTP GET to a URL and shows what came back: the response headers, and where the request spent its time.

The part that makes it more than `curl -I` is `-t`. Point it at one address and the request goes **there** while still carrying the host name the URL asked for, so a CDN edge routes it exactly as it would route real traffic. Walk every A record of a domain that way and you can see which edge is stale, which one is serving a different object, and which one is slow.

**_Useful before pointing a CNAME at a CDN, and for telling one edge apart from the rest afterwards._**

<div align="center">

![overview](https://github.com/ghdwlsgur/gostat/assets/77400522/0661f993-9cda-4382-9fe3-b54bfa5b57ad)

```bash
gostat request https://ghdwlsgur.github.io/ -d
```

</div>

# Why

While inspecting HTTP GET responses with `curl`, the option list kept growing and the command kept getting longer. I had it aliased in my zshrc with the headers I always send. Rather than keep pasting that around, I wanted a tool that already knows those options, in case anyone else has the same problem.

```bash
# what the alias looked like
curl -vo /dev/null -H 'Range:bytes=0-1' --resolve 'naver.com:443:223.130.195.95' 'https://www.naver.com/include/themecast/targetAndPanels.json'

# what it looks like now
gostat request https://www.naver.com/include/themecast/targetAndPanels.json -t 223.130.195.95
```

[Korean Document](https://ghdwlsgur.github.io/docs)

# Installation

### macOS

```bash
# [install]
$ brew tap ghdwlsgur/gostat
$ brew install --cask gostat

# [upgrade]
$ brew update
$ brew upgrade --cask gostat
```

Installed before v1.3.0? It was a formula then and is a cask now, and
`brew upgrade` will not move you across on its own:

```bash
$ brew uninstall --formula gostat
$ brew install --cask gostat
```

### Linux

```bash
$ VERSION=1.3.0

# [install] x86_64
$ curl -fsSL https://github.com/ghdwlsgur/gostat/releases/download/v${VERSION}/gostat_${VERSION}_Linux_x86_64.tar.gz | tar -xz

# [install] arm64
$ curl -fsSL https://github.com/ghdwlsgur/gostat/releases/download/v${VERSION}/gostat_${VERSION}_Linux_arm64.tar.gz | tar -xz

# [execute]
$ ./gostat request https://ghdwlsgur.github.io/
```

The [releases page](https://github.com/ghdwlsgur/gostat/releases) has the current version and the Windows builds.

### Container

Published to GitHub Packages for `linux/amd64` and `linux/arm64`.

```bash
$ docker run --rm ghcr.io/ghdwlsgur/gostat request https://ghdwlsgur.github.io/

# The dashboard draws a terminal UI, so the container needs a terminal.
$ docker run --rm -it ghcr.io/ghdwlsgur/gostat request https://ghdwlsgur.github.io/ -d
```

### From source

```bash
$ go install github.com/ghdwlsgur/gostat@latest
```

# Usage

```
gostat request <url> [flags]
```

| Flag | Short | What it does |
| --- | --- | --- |
| `--target` | `-t` | Address or domain to send the request to instead of resolving the URL. Every A record behind it is probed in turn. |
| `--port` | `-p` | Port to connect to. Defaults to 80 for http and 443 for https. |
| `--host` | `-H` | Host header to send, without changing where the request goes. |
| `--referer` | `-r` | Referer header to send. |
| `--authorization` | `-A` | Authorization header to send. |
| `--dashboard` | `-d` | Draw the live dashboard instead of printing once. |
| `--attack` | `-a` | Keep requesting in a loop, printing only the status code. |
| `--thread` | `-n` | How many workers `-a` runs. |

```bash
# the URL's own A records
$ gostat request https://www.naver.com

# one domain's A records, asking for a URL on another host
$ gostat request https://www.naver.com -t naver.com

# one specific edge
$ gostat request https://www.naver.com -t 223.130.200.104

# an edge that routes on the Host header
$ gostat request https://www.naver.com -t 223.130.200.104 -H www.naver.com

# a referer-protected object
$ gostat request https://www.naver.com/asset.js -t naver.com -r http://naver.com
```

`-d` draws a live view instead of printing once, and the four panels answer four
different questions.

**Status per edge** gives every edge a row of blocks, one per request, coloured
by status class, with the code it answered with last. The row fills from the
left and starts over when it reaches the right, so only the newest block moves
and a change of colour is easy to catch. One edge going bad shows as a band of
a different colour against the rest — no reading required. The legend names
only the classes that have actually come back, so it growing from `2xx` to
`2xx 5xx` is itself the signal.

**Latency** takes the last request apart phase by phase, each bar as wide as the
share of the request that phase took. A request that spends 21ms of its 22ms
waiting on the server says so at a glance.

**Response** is a row per edge and a column per header, so two edges can be
compared line by line. A header only gets a column once some edge has actually
sent it, and keeps it afterwards — a CDN that never sends `Age` or `Via` does
not spend two columns saying so.
**Changes** says whether the answer has been stable: every status code that has
come back, the digest of the body right now, and how often each has moved. An
origin that stamps a request id into its output changes its digest on every
request, so that row counts the changes rather than listing them.

The response table takes the arrow keys, since it is the one panel that can be
wider than the terminal. Its header row and address column stay put while the
rest scrolls, so a column brought into view still says what it is and which
edge it belongs to.

Press `q` or `ctrl-c` to leave. The view lays itself out to the terminal it is
in: a wide window shows a longer run of history, and a narrow one falls back to
short phase names rather than truncating them.

# Reading the output

```
Latency Status
	DNS Lookup          0s              0s
	TCP Connection      324.125µs       324.125µs
	TLS Handshake       1.142417ms      1.466542ms
	Server Processing   21.104625ms     22.571167ms
	Content Transfer    69.916µs        22.641083ms
	Total                               22.901ms
```

The middle column is how long that phase took on its own. The right column is the running total, so the last phase is the sum of everything above it. `Total` is measured separately, from just before the request goes out until the last body byte, so it also covers whatever happens between the phases.

`DNS Lookup` reads `0s` whenever `-t` gave an address to dial: there was no lookup to make, and inventing a number would be worse than reporting none. If a connection came out of the keep-alive pool, the connection phases did not run at all and the report says so rather than leaving three zeros to be misread.

Below the timings are the headers that were sent and the headers that came back, sorted so two runs of the same command can be diffed, followed by the size and SHA-256 of the body. With the default `Range: bytes=0-1` that digest covers two bytes rather than the whole object, which is enough to notice an edge whose content has changed.

# How it works

The request goes out exactly as written. Only the address it is dialled against is replaced, which is what `curl --resolve` does, so the Host line and the TLS SNI still carry the original name.

A few things follow from that:

- **Redirects are not followed.** A 301 from this edge is the answer, not something to chase. Following one would report another host's headers and mix a second connection's timings into the measurement.
- **Certificates are not verified.** An edge holds no certificate for its own address, and checking the chain is not what this tool is for.
- **HTTP/2 is negotiated** where the edge offers it, and the protocol that was actually used is reported.
- **`Range: bytes=0-1` is sent** by default, so a large object is not pulled down just to read its headers. `-a` drops it, because load testing should ask for what a real client would get.

# Compare

#### /etc/hosts

```bash
1.1.1.1 example.com
```

#### gostat

```bash
gostat request https://example.com/test.txt -t 1.1.1.1
```

Instead of editing `/etc/hosts` and remembering to undo it, `gostat` dials `1.1.1.1` for this one request. Nothing on the machine changes, and two edges can be compared side by side in the same shell.

#### curl

```bash
curl -k -I https://185.199.108.153/assets/js/runtime\~main.873fd742.js -H "Host: ghdwlsgur.github.io" -H "Range: bytes=0-1"
```

#### gostat

```bash
gostat request https://ghdwlsgur.github.io/assets/js/runtime\~main.873fd742.js -t 185.199.108.153
```

#### curl

```bash
curl -vo /dev/null -H 'Range:bytes=0-1' --resolve 'naver.com:443:223.130.195.95' 'https://www.naver.com/include/themecast/targetAndPanels.json'
```

#### gostat

```bash
gostat request https://www.naver.com/include/themecast/targetAndPanels.json -t 223.130.195.95
```

If HTTP GET is most of what you reach for `curl` to do, `gostat` says the same thing in fewer characters and reads the timings back for you.

# Development

```bash
$ go test ./... -race        # the suite runs against httptest, never the network
$ go vet ./...
$ bash scripts/deploy.sh release_test   # a full release, built locally, published nowhere
```

| Package | What lives there |
| --- | --- |
| `cmd` | Flags, and wiring the pieces below together |
| `internal/probe` | Sending and measuring one request. Prints nothing |
| `internal/report` | Rendering a result to a writer |
| `internal/dashboard` | Drawing a result with [tview](https://github.com/rivo/tview) |

# License

`gostat` is licensed under the [MIT License](./LICENSE).
