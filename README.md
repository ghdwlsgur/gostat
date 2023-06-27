<div align="center">

<br>
<br>

<img width="50%" alt="govpn-logo" src="https://user-images.githubusercontent.com/77400522/204132452-9c0182e1-860f-4c79-87f9-a18c68e2de53.png">

![GitHub tag (latest SemVer)](https://img.shields.io/github/v/tag/ghdwlsgur/gostat?color=success&label=version&sort=semver)
[![Go Report Card](https://goreportcard.com/badge/github.com/ghdwlsgur/gossl)](https://goreportcard.com/report/github.com/ghdwlsgur/gostat)
[![Codacy Badge](https://app.codacy.com/project/badge/Grade/4d8e0ef64d1348c19d0ccbae23290eb7)](https://www.codacy.com/gh/ghdwlsgur/gostat/dashboard?utm_source=github.com&utm_medium=referral&utm_content=ghdwlsgur/gostat&utm_campaign=Badge_Grade)
[![Maintainability](https://api.codeclimate.com/v1/badges/1d8e562559047191efd8/maintainability)](https://codeclimate.com/github/ghdwlsgur/gostat/maintainability)
[![circle ci](https://circleci.com/gh/ghdwlsgur/gostat.svg?style=svg)](https://circleci.com/gh/ghdwlsgur/gostat)

</div>

# Overview

This is an interactive CLI tool that uses the net/http package to make an HTTP GET request to a specific URL and checks the latency step by step. It also displays brief information on request and response headers.

**_It can be useful for testing purposes before transferring to a CNAME record of a domain that uses a CDN domain._**

<div align="center">

[![asciicast](https://asciinema.org/a/yakWtqPyYdwlFHuEEHhcGLHIq.svg)](https://asciinema.org/a/yakWtqPyYdwlFHuEEHhcGLHIq)

</div>

# Why

The motivation for creating this tool was that while using the curl command to inspect HTTP GET responses, I found that the options I used became increasingly varied, and the command itself became longer. Personally, I had customized the command with the options I frequently used along with request headers in my zshrc script. However, instead of using this method, I wanted to create a tool with fixed options and request header values that I typically use. I wanted to create a tool that could be useful for others who may have similar concerns or team members who could benefit from it.

[Korean Document](https://ghdwlsgur.github.io/docs)

```bash
curl -vo /dev/null -H 'Range:bytes=0-1' --resolve 'naver.com:443:223. 130.195.95' 'https://www.naver.com/include/themecast/targetAndPanels.json'
```

# Installation

### homebrew

```bash

# [install]
brew tap ghdwlsgur/gostat
brew install gostat

# [upgrade]
brew upgrade gostat
```

### [Download](https://github.com/ghdwlsgur/gostat/releases)

# Compare

#### /etc/hosts

```bash
1.1.1.1 example.com
```

#### gostat

```bash
gostat request https://example.com/test.txt -t 1.1.1.1
```

The gostat command does not make a request to the host IP of example.com, but rather maps it to `1.1.1.1` in the `/etc/hosts` file and then queries DNS for the host IP. This allows you to check the response code and response headers for the HTTP GET method request.

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

If you primarily use the `HTTP GET method`, `gostat` may be more intuitive and convenient to use than the `curl` command, which behaves in the same way.

# How to use

**_Simple_**

```bash
gostat request [URL]

# Example
gostat request https://www.naver.com
```

**_Target (domain / ip)_**

```bash
gostat request [URL] -t [Target(domain or ip)]

# Example
gostat request https://www.naver.com -t naver.com
gostat request https://www.naver.com -t 223.130.200.104
```

**_Request Header (Referer)_**

```bash
gostat request [URL] -t [Target] -r [Referer]

# Example
gostat request https://www.naver.com -t naver.com -r http://naver.com
```

**_Request Header (Host)_**

```bash
gostat request [URL] -t [Target] -H [Host]

# Example
gostat request https://www.naver.com -H naver.com
```

**_Request Header (Authorization)_**

```bash
gostat request [URL] -A [Authorization]
```

# License

gossl is licensed under the [MIT](https://github.com/ghdwlsgur/gostat/blob/master/LICENSE)
