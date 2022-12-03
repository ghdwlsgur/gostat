<div align="center">

<br>
<br>

<img width="50%" alt="govpn-logo" src="https://user-images.githubusercontent.com/77400522/204132452-9c0182e1-860f-4c79-87f9-a18c68e2de53.png">

![GitHub tag (latest SemVer)](https://img.shields.io/github/v/tag/ghdwlsgur/gostat?color=success&label=version&sort=semver)
[![Go Report Card](https://goreportcard.com/badge/github.com/ghdwlsgur/gossl)](https://goreportcard.com/report/github.com/ghdwlsgur/gostat)
[![Codacy Badge](https://app.codacy.com/project/badge/Grade/4d8e0ef64d1348c19d0ccbae23290eb7)](https://www.codacy.com/gh/ghdwlsgur/gostat/dashboard?utm_source=github.com&utm_medium=referral&utm_content=ghdwlsgur/gostat&utm_campaign=Badge_Grade)
[![Maintainability](https://api.codeclimate.com/v1/badges/1d8e562559047191efd8/maintainability)](https://codeclimate.com/github/ghdwlsgur/gostat/maintainability)

</div>

# Overview

An Interactive CLI Tool that Receives the response of the URL to each "A record" of the target domain to the url using the http or https protocol.

# Why

I had to proxy to the origin domain's A record address to get a response from the target domain's content, as in the example below.

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

# How to use

### `request`

```bash
gostat request https://www.naver.com -t naver.com
```

<div align="center">
<img src="https://user-images.githubusercontent.com/77400522/205435663-8921c405-58ea-4452-9f45-58e4d3b90c5a.png">
</div>

### `request` - add referer

```
gostat requet https://www.naver.com -t naver.com -r http://naver.com
```

<div align="center">
<img src="https://user-images.githubusercontent.com/77400522/205435871-8d021d37-e2ad-4d30-8d52-c382d28859d3.png">
</div>

# License

gossl is licensed under the [MIT](https://github.com/ghdwlsgur/gostat/blob/master/LICENSE)
