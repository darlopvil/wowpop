# wowpop

A privacy-friendly frontend for [Wallapop](https://es.wallapop.com/) Spain.
Browse second-hand listings without needing JavaScript, without trackers and
without your browser ever talking to Wallapop.

This repository is the source code of a private instance, built for personal
use. No public instance is offered. Issues are welcome.

## Features

- No JavaScript: everything works with scripts turned off.
- Your browser never contacts Wallapop or any third party: styles, fonts and
  icons are served by the instance itself.
- No accounts, no cookies and no trackers.
- Wallapop receives no identifier of your device: no cookies, no device id,
  no app version.
- Lightweight: the first visit downloads about 85 KB in all, more than half
  of it the font. After that a page is a few KB, because styles, drawings
  and the font stay in the browser for a year. Styles and drawings are sent
  gzipped, compressed once when wowpop starts.
- A dark Frutiger Aero look, in the spirit of Windows Vista and 7.

The interface is in Spanish, like the Wallapop website it mirrors.

## How it works

wowpop is a single Go binary with no dependencies outside the standard
library. Templates, styles and fonts are embedded in it. It talks to the
same two Wallapop hosts the website uses:

| Host | Used for |
|---|---|
| `api.wallapop.com` | searches, filters and a seller's items |
| `es.wallapop.com` | listing and profile data, from the website's own Next.js data routes |
| `cdn.wallapop.com` | photos, passed on through wowpop |

Requests carry a browser `User-Agent`, `Accept-Language: es-ES` and
`X-DeviceOS: 0`, the one header the API requires. Redirects are only
followed within those hosts, and responses over 2 MiB are refused.

To check from the server that wowpop gets the same answers as the Wallapop
website, run a probe:

```sh
docker exec wowpop_app /wowpop -sonda busqueda="game boy"
docker exec wowpop_app /wowpop -sonda ficha=<listing slug>
docker exec wowpop_app /wowpop -sonda vendedor=<profile slug>
```

It prints the status, size and time of each request and a few fields of the
answer. Add `-crudo` to also get the raw JSON on stdout, to map fields from
real responses.

## Running it

### Requirements

- Docker, or Go 1.26 or later.
- Outbound HTTPS to `api.wallapop.com`, `es.wallapop.com` and `cdn.wallapop.com`.

### With Docker

```sh
git clone https://github.com/darlopvil/wowpop.git
cd wowpop
cp docker-compose.example.yml docker-compose.yml
docker compose up -d --build
```

The image is built from `scratch` and runs as an unprivileged user. The
example compose file runs it read-only, with every capability dropped, a
32 MiB Go memory limit (`GOMEMLIMIT`) and a 64 MB container limit.

It publishes no ports: it expects a reverse proxy that reaches the
container by name on a shared Docker network (`proxy_network` in the
example; rename it to yours). That network cannot be `internal`, because
wowpop needs outbound HTTPS to Wallapop. To try it without a proxy,
uncomment the `ports:` lines.

### Without Docker

```sh
go build -trimpath -o wowpop .
./wowpop -p 8080
```

### Configuration

| Variable | Flag | Default | |
|---|---|---|---|
| `WOWPOP_HOST` | `-h` | `0.0.0.0` | Listen address |
| `WOWPOP_PORT` | `-p` | `8080` | Listen port |

Flags win over variables.

### Routes

| Route | |
|---|---|
| `/` | home page; it never talks to Wallapop |
| `/search?keywords=…` | search results; `orden=nuevos`, `baratos` or `caros` changes the order, and `b` and `p` open a numbered page of a search wowpop remembers |
| `/item/…` | a listing, at the same path as on the website, so a redirect only changes the host |
| `/foto/…` | photos from Wallapop's image server, at the widths the website uses |
| `/static/…` | styles, fonts and icons |
| `/about`, `/robots.txt`, `/favicon.ico` | |

### Behind a reverse proxy

wowpop serves plain HTTP and leaves TLS to the proxy. It sets its own
security headers, including a Content Security Policy that allows no
scripts at all, so the proxy does not need to add any.

`robots.txt` asks every crawler to stay away.

## Development

```sh
go vet ./...
go test ./...
```

The tests run against local fake servers; they never reach Wallapop.

## Legal notice

wowpop is an independent, non-commercial, personal project. It is **not
affiliated with, endorsed by or sponsored by Wallapop**. "Wallapop" is a
trademark of its owner and is used here only to describe what this software
works with.

What wowpop does: when you search or open a listing, it requests the same
publicly available data that the official Wallapop website loads in an
ordinary browser for a visitor without an account, and renders it as plain
HTML, without trackers, ads or JavaScript.

What wowpop does not do:

- It does not use or require accounts, and it does not get around logins,
  paywalls, CAPTCHAs or any other access control.
- It does not decompile or modify any app; it only uses the requests the
  official website makes in a normal browser.
- It does not crawl, mirror or redistribute the catalogue, and it keeps no
  copy of it: no database and no files. Pages are cached in memory for a
  short time to avoid repeating identical requests, and the cache is lost
  on restart. Photos may be kept by your browser for as long as Wallapop's
  own image servers allow.
- It does not register views or favourites, and it sends Wallapop no device
  identifier.
- It does not use Wallapop's third-party location service: locations are
  resolved locally.
- It stores no personal data about sellers, and it only shows what the
  listing itself publishes.

Listings, photos and descriptions belong to the people who published them
and are shown as they come, on demand.

If you run your own instance, you are responsible for complying with the
laws of your country and the terms of the services involved.

If you represent Wallapop and have a concern about this project, please open
an issue and it will be looked at in good faith.

## Licences

wowpop is free software: you can redistribute it and/or modify it under the
terms of the GNU Affero General Public License, version 3 or (at your
option) any later version. It is provided "as is", without warranty of any
kind. See [LICENSE](LICENSE).

The Open Sans font is © The Open Sans Project Authors and is licensed under
the SIL Open Font License 1.1 ([static/fuentes/OFL-OpenSans.txt](static/fuentes/OFL-OpenSans.txt)).

The hill, the aurora ribbons and the bubbles were drawn for wowpop and are
under the AGPL like the rest of the code. The other drawings come from
[Openclipart](https://openclipart.org/) and are in the public domain (CC0);
thanks to their authors all the same:

| File | Drawing | Author |
|---|---|---|
| `static/arte/mariposa-1.svg`, `mariposa-2.svg` | [Five Butterflies](https://openclipart.org/detail/346015) | j4p4n |
| `static/arte/pez-dorado.svg` | [Fish icons](https://openclipart.org/detail/181437) | vectorsme |
| `static/arte/pez-angel.svg` | [Angel Fish](https://openclipart.org/detail/122389) | cgillis73 |
| `static/arte/brote.svg`, and the sprouts in `paisaje.svg` | [Green Leaves Graphics](https://openclipart.org/detail/237408) | GoodFreeArt |
| `static/arte/info.svg` | [Small blue 'i' info button](https://openclipart.org/detail/7982) | TzeenieWheenie |
| `static/arte/pergamino.svg` | [Blank Scroll](https://openclipart.org/detail/351206) | j4p4n |
| `static/arte/check.svg` | [tasto 7](https://openclipart.org/detail/34723) | francesco_rollandin |
| `static/arte/tierra-reciclaje.svg` | [crystal earth recycle](https://openclipart.org/detail/2213) | kuba |
| `static/arte/tierra.svg` | [Earth](https://openclipart.org/detail/3320) | barretr |

They were cleaned up for the web (metadata and editor data removed, shadows
dropped from the butterflies, paths simplified), which CC0 allows.
