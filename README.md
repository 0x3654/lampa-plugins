<div align="center">

# 🔌 Lampa Plugins

**Plugins for the [Lampa](https://lampa.mx) media app and the tracker-top backend**

[![License: MIT](https://img.shields.io/badge/License-MIT-0x3654.svg)](LICENSE)
[![Pages](https://img.shields.io/badge/install-GitHub%20Pages-222222.svg)](https://0x3654.github.io/lampa-plugins/)
[![Docker](https://img.shields.io/badge/tracker--top-ghcr.io%2F0x3654-2496ED.svg)](https://github.com/0x3654/lampa-plugins/pkgs/container/tracker-top)
[![Go](https://img.shields.io/badge/Go-1.27-00ADD8.svg)](https://go.dev)

[Install](#install) · [Top](#top) · [Torrent Send](#torrent-send) · [t.js](#tjs-bootstrap) · [tracker-top](#tracker-top) · [Development](#development) · [По-русски](#по-русски)

</div>

## Install

In Lampa: Settings → Extensions → «+», paste a URL (per device; on iPhone the
PWA is separate from the Safari tab):

| Plugin | URL |
|---|---|
| Top | `https://0x3654.github.io/lampa-plugins/top.js` |
| Torrent Send | `https://0x3654.github.io/lampa-plugins/transmission-send.js` |
| Plex Sync | `https://0x3654.github.io/lampa-plugins/plex-sync.js` |
| everything at once | `https://0x3654.github.io/t.js` (bootstrap, see below) |

## Top

Two menu items instead of the default feed — this is the main plugin.

### Top · TMDB

TMDB popularity screens for movies and series — three flavours each:
weekly trending (what's hot now), best rated (all-time classics) and
new releases of 2025+ (six variants; the day/window tops were dropped —
they overlapped the weekly trending by 78–100% and the premiere windows
held 8–15 films). Switch the variant with the **right** button on the
screen. Cards and pagination are native, requests go through the Lampa
API (your TMDB proxy setting applies), so screens open instantly.

### Top · trackers

Release tops of NNM Club + rutor (video categories only), enriched with TMDB
posters and metadata. A click opens the native film page — quality and voice
are picked later in the Lampa parser, so releases of the same film collapse
into a single position. Sorting (right button): by seeders **now** or
**all-time classics** (NNM `o=6`, rutor doesn't take part in that mode).
Releases that don't match TMDB (software, games, packs) are not shown.

The filter sheet (right button) mirrors the native torrent-filter UI:
quality, up to two voices, toggles, «Show».

### Settings

Settings → Top:

| Setting | What it does |
|---|---|
| Top server address | tracker-top URL (a working one: `https://top.0x3654.com`) |
| Top as home screen | open the last used Top variant on start instead of the feed |
| Min quality (trackers) | Any / 720p+ / 1080p+ / 2160p+ |
| Voice 1 / Voice 2 (trackers) | voice plates to show (e.g. Dub + Multi) |
| Hide watched | exclude watched by Lampa history, server-side |
| With releases only | in Top · TMDB show only films that have a tracker release under our filters |
| Hide series | Top · trackers: movies only |
| Russian titles only | hide releases/cards with no cyrillic in the title |
| Hide anime | Top · trackers: no Japanese anime, cartoons stay (on by default) |
| Hide CAM/TS | camrips and TS sound are cut before dedup: a film with only such releases disappears entirely, one with a proper release stays (on by default) |

## Torrent Send

Two ways to send a release from the long-press menu (any tracker — the parser
magnet goes as-is):

| Menu item | When shown | What it does |
|---|---|---|
| **Copy magnet** | magnet present | magnet to clipboard |
| **Open magnet** | magnet present | `window.location = magnet:` → the system handler (a torrent client or a helper app) |
| **Add to Plex** (always last) | magnet present | pushes the release into the personal «auto» feed of [nnm-rss](https://github.com/0x3654/nnm-rss), from which transmission-rss downloads it into the Plex library; duplicate releases answer «already in the feed» |

The former nnm-auto.js plugin is merged into Torrent Send; the file remains a
stub — remove it if it was ever installed.

> [!NOTE]
> Transmission 4.1.3 removed CORS headers from its RPC, so browser/PWA
> clients cannot talk to it directly. That is why Torrent Send routes through
> system `magnet:` handlers and native clients instead of `fetch`.

macOS has no UI for URL schemes; the handler is switched via LaunchServices
(TRG here as an example, bundle id `com.transgui`):

```bash
defaults write com.apple.LaunchServices/com.apple.launchservices.secure LSHandlers \
  -array-add '<dict><key>LSHandlerURLScheme</key><string>magnet</string>\
<key>LSHandlerRoleAll</key><string>com.transgui</string></dict>'
killall lsd
```

Check: `open 'magnet:?xt=urn:btih:0000000000000000000000000000000000000000'`.
Bundle id of any app: `osascript -e 'id of app "Name"'`.

### Add to Plex — credentials

Settings → NNM RSS (nnm.0x3654.com):

| Setting | What it does |
|---|---|
| Server address | nnm-rss URL (prefilled `https://nnm.0x3654.com`) |
| Login / Password | an account on the service (register on its front page); the password stays in device localStorage and is masked with dots |
| Check login | button: logs in with the current credentials, status right in the row (✓ green / ✗ red) |

Authorization: JSON login → token (cached per server+login), then
`Authorization: Bearer`; on 401 (a password change rotates the token) it
re-logins and retries once.

## Plex Sync

Two-way watch status with a Plex account (Settings → Plex). Linking is a
plex.tv/link code (or a manual X-Plex-Token); the account's server is
detected automatically, a manual LAN address is optional. Nothing is sent
until an account is linked.

| Direction | What happens |
|---|---|
| Lampa → Plex | finished (≥ 90%) — scrobble on the server; partial — playback position; unmarked — unscrobble |
| Plex → Lampa | «Sync now» pulls watched items and positions into the Lampa timeline (progress on cards) and «Viewing history» |

Matching is exact, by TMDB guids (`tmdb://id` — the Plex Movie/Series
agents carry them). A local mark newer than the incoming one is never
overwritten (LWW). With «Sync Watch State» enabled on the account, a mark
on one server propagates to the others by itself.

## t.js bootstrap

One short URL for a fresh Lampa install: adds the plugins above (plus the etor
torrent unlock and tmdb-proxy), applies a curated settings block once
(version-gated), picks the TorrServer link by probing, migrates old plugin
URLs after the repository rename and self-heals the Apple TV platform
detection glitch. Source of truth lives in the user-site repo; the copy here
mirrors it.

## tracker-top

The Go service behind «Top · trackers» and the server-side TMDB screens: one
static binary in a scratch image (~11 MB). Sources: NNM Club
(`tracker.php?o=10`, the video category subtree parsed from the page) and
rutor (per-category tops). Philosophy: **films for browsing, not a release
feed** — releases of the same film collapse into one position.

```
GET /top?src=both|nnm|rut&cat=video|all&pages=1..3&minq=720|1080|2160&audio=all|dub|no_ts
GET /feed?...   server-side TMDB-top assembly with filters and watched-exclude
GET /healthz
```

In-memory cache (TTL 600 s), CORS `*`, mirror override via `NNM_BASE` /
`RUTOR_BASE`. CI builds a multi-arch image: `ghcr.io/0x3654/tracker-top`
(deps → test → build → scratch; a broken parser fails the build; non-root,
healthcheck). A public instance serves `https://top.0x3654.com`.

## Development

**One feature — one commit.** Everything is developed and tested locally
against a local Lampa; `main` is production (Pages serves the plugins, CI
builds the tracker-top image), so the push happens once, after all tests and
a live check — never as a stream of intermediate fixups.

Tests run in containers (host stays clean):

```bash
docker run --rm -v "$PWD":/src -w /src node:24-alpine node smoke-top.js
docker run --rm -v "$PWD":/src -w /src node:24-alpine node smoke-test.js
docker run --rm -v "$PWD"/tracker-top:/src -w /src golang:1.27-alpine go test ./...
```

### Local loop

Two containers on the Mac; other devices on the LAN reach it by the Mac's
Bonjour name (`scutil --get LocalHostName` → `<mac-name>.local`). The nginx
config with the same-origin `/topapi` proxy is in the repo —
[dev/nginx.conf](dev/nginx.conf).

```bash
# tracker-top under test (:8355)
docker build -t tracker-top:dev tracker-top
docker run -d --name trackertop-dev -p 8355:8355 \
    -v trackertop-dev-cache:/data tracker-top:dev

# local Lampa (:8098): official webroot + nginx conf, once:
git clone --depth 1 https://github.com/yumata/lampa ~/code/lampa
rm -rf ~/code/lampa/.git   # disposable static webroot

docker run -d --name lampa-dev -p 8098:80 \
  -v "$HOME/code/lampa":/usr/share/nginx/html:ro \
  -v "$PWD/dev/nginx.conf":/etc/nginx/conf.d/default.conf:ro \
  nginx:1.27-alpine

# sync plugin edits into the webroot (after every edit; single-file bind
# mounts would pin the old inode and serve stale code):
cp top.js transmission-send.js t.js plex-sync.js ~/code/lampa/
```

Optional: route the tracker-top container's outbound traffic (Jackett
proxy, trackers, TMDB) through a VPS — the Mac's IP collects rate limits
fast during heavy debugging. A plain SSH SOCKS tunnel is enough; Go's
HTTP stack honours a socks5 proxy from the environment:

```bash
ssh -fN -D 127.0.0.1:1080 <vps>        # any VPS of yours
docker run -d --name trackertop-dev -p 8355:8355 -v trackertop-dev-cache:/data \
    -e HTTP_PROXY=socks5://host.docker.internal:1080 \
    -e HTTPS_PROXY=socks5://host.docker.internal:1080 \
    -e NO_PROXY=localhost,127.0.0.1 \
    tracker-top:dev
```

One-time, after cloning the webroot, inject the dev hook into
`~/code/lampa/index.html` (before the `var lampa_url` script) — it flags
the bootstrap for dev and loads the local `/t.js`:

```html
<script>
    window.TJS_DEV = true
    ;(function waitLampa(n){
        if(window.appready || window.Lampa || n > 400){
            var s = document.createElement('script')
            // bust: even a cached page always fetches a fresh t.js
            s.src = '/t.js?v=' + Math.floor((new Date()).getTime() / 6e4)
            document.head.appendChild(s)
        } else setTimeout(function(){ waitLampa(n + 1) }, 50)
    })(0)
</script>
```

Open the page and the bootstrap does the rest:

- **Mac browser**: open `http://localhost:8098` — t.js installs the plugins
  from the same origin and points the Top server at the same-origin
  `/topapi` proxy automatically (dev sets it on every boot), then reloads
  once. Nothing to configure by hand.
- **Apple TV**: native Lampa → «Подмена адреса» →
  `http://<mac-name>.local:8098` — both addresses are derived from the page
  address, so the same setup works from any device (`localhost` on the TV
  is the TV itself).
- Plain **http is deliberate**: an https Lampa page cannot load http plugins
  or call a http API (mixed content), so a local Lampa is the only way to
  test a local backend. On an http page Lampa's `protocol()` helper defaults
  to `https` (empty `protocol` storage) and fires TLS handshakes into the
  http nginx — the dev bootstrap forces `protocol=http` on every boot.
- Lampa loads plugins at startup — after editing a plugin, restart the
  app (or reload the page).

Notes:

- Headless e2e: `mcr.microsoft.com/playwright:v1.49.0-jammy` (the tag with
  `.0`), the playwright module installed into `/work` of the container,
  scripts mounted at `/work`; the container reaches the Mac as
  `host.docker.internal:8098`.
- The NNM top URL must carry the `/forum/` prefix:
  `nnmclub.to/forum/tracker.php?o=10` (200; without `/forum/` — 404).

Deploy: push to `main` → Pages publishes the plugins, CI builds the image;
the public instance updates from the registry.

Extension points (verified against the Lampa 3.3.3 sources): hooks
`Lampa.Listener('torrent'/'torrent_file', onlong)` push menu items into
`e.menu`; screens use `Lampa.InteractionCategory` (the collections-plugin
reception) + `Lampa.Api.sources.tmdb.get`; menus `Lampa.Menu.addButton`,
settings `Lampa.SettingsApi`.

> [!IMPORTANT]
> Unofficial hobby project, no affiliation with Lampa, NNM Club or rutor.
> Tracker parsing follows guest-accessible pages and may break on redesigns.

---

# По-русски

## Установка

В Lampa: Настройки → Расширения → «+», вставить URL (на каждом устройстве
отдельно; PWA на iPhone — отдельно от вкладки Safari):

| Плагин | URL |
|---|---|
| Top | `https://0x3654.github.io/lampa-plugins/top.js` |
| Torrent Send | `https://0x3654.github.io/lampa-plugins/transmission-send.js` |
| Plex Sync | `https://0x3654.github.io/lampa-plugins/plex-sync.js` |
| всё сразу | `https://0x3654.github.io/t.js` (бутстрап, см. ниже) |

## Top — «адекватный топ» вместо ленты

Два пункта в главном меню — основной плагин.

### Топ · TMDB

Популярное/лучшее по TMDB — по три смысла на фильмы и сериалы: тренды недели
(горячее сейчас), лучшее по рейтингу (классика), новинки 2025+ (широкий
каталог свежего). Шесть вариантов; «за день» и окна 14/30 дней убраны — они
совпадали с трендами недели на 78–100%, а окна премьер вмещали 8–15 фильмов.
Переключение варианта — кнопка **вправо** на экране. Карточки
и пагинация нативные, запросы идут через API Lampa (учитывается прокси TMDB
из настроек), экраны открываются мгновенно.

### Топ · трекеры

Топ раздач NNMClub/RUTOR, только видео-категории, обогащённый постерами и
метаданными TMDB. Клик по карточке — обычная страница фильма в Lampa:
качество и озвучку выбираешь дальше в парсере, поэтому раздачи одного фильма
схлопываются в одну позицию. Сортировка (кнопка вправо): по сидам «сейчас»
или «классика» — завершённость за всё время (NNM `o=6`, rutor в этом режиме
не участвует). Раздачи, не сматчившиеся с TMDB (софт, игры, сборники), не
показываются.

Лист фильтров (кнопка вправо) повторяет штатный фильтр торрентов: качество,
до двух озвучек, тумблеры, «Показать».

### Настройки

Настройки → Топ:

| Параметр | Что делает |
|---|---|
| Адрес сервера топа | URL tracker-top (рабочий — `https://top.0x3654.com`) |
| «Топ» вместо главной | при запуске открывается последний вариант «Топа» вместо ленты |
| Мин. качество (трекеры) | Любое / 720p+ / 1080p+ / 2160p+ |
| Озвучка 1 / Озвучка 2 (трекеры) | какие плашки озвучки показывать (напр. Дубляж + Многоголосый) |
| Скрывать просмотренные | исключать просмотренное по истории Lampa, на сервере |
| Только с раздачами | в «Топе · TMDB» показывать только фильмы, у которых на трекерах есть раздача под наши фильтры |
| Скрыть сериалы | «Топ · трекеры»: только фильмы |
| Только на русском | скрывать раздачи/карточки совсем без русских букв в названии |
| Скрыть аниме | «Топ · трекеры»: японские аниме прочь, мультики остаются (вкл по умолчанию) |
| Скрывать CAM/TS | камрипы и «звук с TS» вычищаются ДО дедупа: фильм только с такими раздачами исчезает целиком, с нормальной — остаётся с ней (вкл по умолчанию) |

## Torrent Send — отправка в торрент-клиент

Два способа отправить раздачу из меню долгого нажатия (работает с любым
трекером — магнет парсера уходит как есть):

| Пункт | Когда показывается | Что делает |
|---|---|---|
| **Скопировать магнет** | есть магнет | магнет в буфер обмена |
| **Открыть магнет** | есть магнет | `window.location = magnet:` → системный обработчик схемы |
| **Добавить в Plex** (всегда последним) | есть магнет | раздача → личная лента «авто» [nnm-rss](https://github.com/0x3654/nnm-rss) → transmission-rss качает в библиотеку Plex; повторное добавление — «Уже в ленте» |

Бывший плагин nnm-auto.js влит в Torrent Send; файл остаётся заглушкой — на
устройствах, где он был установлен, удалите расширение.

> [!NOTE]
> Transmission 4.1.3 убрал CORS-заголовки из RPC, поэтому браузер/PWA не могут
> обращаться к нему напрямую — Torrent Send идёт через системные обработчики
> `magnet:` и нативные клиенты, а не через `fetch`.

Системного UI для схем на macOS нет; обработчик переключается записью в
LaunchServices (TRG для примера, bundle id `com.transgui`):

```bash
defaults write com.apple.LaunchServices/com.apple.launchservices.secure LSHandlers \
  -array-add '<dict><key>LSHandlerURLScheme</key><string>magnet</string>\
<key>LSHandlerRoleAll</key><string>com.transgui</string></dict>'
killall lsd
```

Проверить: `open 'magnet:?xt=urn:btih:0000000000000000000000000000000000000000'`.
Bundle id любого приложения: `osascript -e 'id of app "Имя"'`.

### «Добавить в Plex» — доступы

Настройки → NNM RSS (nnm.0x3654.com):

| Параметр | Что делает |
|---|---|
| Адрес сервера | URL сервиса nnm-rss (предзаполнен `https://nnm.0x3654.com`) |
| Логин / Пароль | аккаунт на сервисе (регистрация на его главной); пароль лежит в localStorage устройства и в настройках скрыт точками |
| Проверить вход | кнопка: логинится текущим логином/паролем, статус прямо в строке (✓ зелёным / ✗ красным) |

Авторизация: JSON-логин → токен (кэшируется на пару сервер+логин), далее
`Authorization: Bearer`; при 401 (смена пароля ротирует токен) — перелогин и
один повтор.

## Plex Sync — статус просмотра

Двусторонний статус просмотра с аккаунтом Plex (Настройки → Plex).
Привязка — кодом plex.tv/link (или ручным X-Plex-Token); сервер аккаунта
определяется сам, ручной LAN-адрес — опционально. До привязки аккаунта
ничего не отправляется.

| Направление | Что происходит |
|---|---|
| Lampa → Plex | досмотрели (≥ 90%) — scrobble на сервере; частичный просмотр — позиция; сняли отметку — unscrobble |
| Plex → Lampa | «Синхронизировать» переносит просмотренное и позиции в таймлайн Lampa (прогресс на карточках) и «Историю просмотров» |

Матчинг точный, по TMDB-гуидам (`tmdb://id` — их несут агенты Plex
Movie/Series). Локальная отметка новее входящей не затирается (LWW). С
включённым «Sync Watch State» у аккаунта отметка на одном сервере сама
разъедется по остальным.

## t.js — бутстрап

Один короткий URL для чистой установки Lampa: ставит плагины выше (плюс etor
— разблокировщик торрентов — и tmdb-proxy), один раз применяет кураторский
блок настроек (version-gated), выбирает ссылку TorrServer пробой, мигрирует
старые URL плагинов после переименования репозитория и самолечит глюк
детекции платформы Apple TV. Источник правд — репо user-site; копия здесь —
зеркало.

## tracker-top — сервер топов

Go-сервис экранов «Топ · трекеры» и серверной сборки TMDB-топов: один
статический бинарник в scratch-образе (~11 МБ). Источники: NNMClub
(`tracker.php?o=10`, видео-поддерево категорий парсится со страницы) и rutor
(топы по разделам). Философия: **фильмы для просмотра, а не лента раздач** —
раздачи одного фильма схлопываются в одну позицию.

```
GET /top?src=both|nnm|rut&cat=video|all&pages=1..3&minq=720|1080|2160&audio=all|dub|no_ts
GET /feed?...   серверная сборка TMDB-топов с фильтрами и exclude просмотренного
GET /healthz
```

Кэш в памяти (TTL 600 с), CORS `*`, зеркала — env `NNM_BASE` / `RUTOR_BASE`.
CI собирает multi-arch образ `ghcr.io/0x3654/tracker-top` (deps → test →
build → scratch; сломанный парсер валит сборку; non-root, healthcheck).
Публичный инстанс: `https://top.0x3654.com`.

## Разработка

**Одна фича — один коммит.** Разработка и тестирование — локально, против
локальной лампы; `main` — прод (Pages отдаёт плагины устройствам, CI собирает
образ tracker-top), поэтому пуш — один, после всех тестов и живой проверки,
а не поток промежуточных доводок.

Тесты — в контейнерах (хост не пачкаем):

```bash
docker run --rm -v "$PWD":/src -w /src node:24-alpine node smoke-top.js
docker run --rm -v "$PWD":/src -w /src node:24-alpine node smoke-test.js
docker run --rm -v "$PWD"/tracker-top:/src -w /src golang:1.27-alpine go test ./...
```

### Локальный контур

Два контейнера на маке; остальные устройства в LAN видят его по Bonjour-имени
мака (`scutil --get LocalHostName` → `<имя-мака>.local`). Конфиг nginx с
same-origin-прокси `/topapi` — в репо: [dev/nginx.conf](dev/nginx.conf).

```bash
# tracker-top под тестом (:8355)
docker build -t tracker-top:dev tracker-top
docker run -d --name trackertop-dev -p 8355:8355 \
    -v trackertop-dev-cache:/data tracker-top:dev

# локальная лампа (:8098): официальный вебрут + конфиг nginx, один раз:
git clone --depth 1 https://github.com/yumata/lampa ~/code/lampa
rm -rf ~/code/lampa/.git   # вебрут расходный

docker run -d --name lampa-dev -p 8098:80 \
  -v "$HOME/code/lampa":/usr/share/nginx/html:ro \
  -v "$PWD/dev/nginx.conf":/etc/nginx/conf.d/default.conf:ro \
  nginx:1.27-alpine

# синхронизация правок плагинов в вебрут (после каждой правки; одиночные
# bind-mount файлов цепляют старый inode и отдавали бы устаревший код):
cp top.js transmission-send.js t.js plex-sync.js ~/code/lampa/
```

Опционально: вывод исходящего трафика tracker-top (Jackett-прокси, трекеры,
TMDB) через свой VPS — IP мака при интенсивной отладке быстро набирает
рейт-лимиты. Достаточно чистого SSH-SOCKS-туннеля — HTTP-стек Go понимает
socks5-прокси из окружения:

```bash
ssh -fN -D 127.0.0.1:1080 <vps>        # любой свой VPS
docker run -d --name trackertop-dev -p 8355:8355 -v trackertop-dev-cache:/data \
    -e HTTP_PROXY=socks5://host.docker.internal:1080 \
    -e HTTPS_PROXY=socks5://host.docker.internal:1080 \
    -e NO_PROXY=localhost,127.0.0.1 \
    tracker-top:dev
```

Однократно после клона вшить в `~/code/lampa/index.html` дев-хук (перед
скриптом с `var lampa_url`) — он помечает бутстрап дев-режимом и грузит
локальный `/t.js`:

```html
<script>
    window.TJS_DEV = true
    ;(function waitLampa(n){
        if(window.appready || window.Lampa || n > 400){
            var s = document.createElement('script')
            // бустер: даже кэшированная страница возьмёт свежий t.js
            s.src = '/t.js?v=' + Math.floor((new Date()).getTime() / 6e4)
            document.head.appendChild(s)
        } else setTimeout(function(){ waitLampa(n + 1) }, 50)
    })(0)
</script>
```

Дальше открываешь страницу — бутстрап делает всё сам:

- **Браузер мака**: `http://localhost:8098` — t.js ставит плагины с того же
  origin, адрес сервера Топа — same-origin прокси `/topapi` (в dev
  прописывается на каждом старте), одна перезагрузка. Руками настраивать
  ничего не надо.
- **Apple TV**: нативная лампа → «Подмена адреса» →
  `http://<имя-мака>.local:8098` — оба адреса выводятся из адреса страницы,
  так что с любого устройства одно и то же (`localhost` на TV — это сам TV).
- **http — принципиально**: https-страница лампы не может грузить http-плагины
  и ходить в http-API (mixed content), поэтому локальный бэкенд тестируется
  только через локально открытую лампу. На http-странице хелпер лампы
  `protocol()` дефолтит в `https` (storage `protocol` пуст) и шлёт
  TLS-хендшейки в http-nginx — дев-бутстрап форсит `protocol=http` на
  каждом старте.
- Лампа грузит плагины на старте — после правки плагина перезапустить
  приложение (или перезагрузить страницу).

Заметки:

- Headless-e2e: `mcr.microsoft.com/playwright:v1.49.0-jammy` (тег именно
  с `.0`), модуль playwright ставить в `/work` контейнера, скрипты монтировать
  в `/work`; контейнер ходит на мак как `host.docker.internal:8098`.
- Топ-URL NNM обязан быть с префиксом `/forum/`:
  `nnmclub.to/forum/tracker.php?o=10` (200; без `/forum/` — 404).

Деплой: пуш в `main` → Pages публикует плагины, CI собирает образ; публичный
инстанс обновляется из registry.

Точки расширения (сверены по исходникам Lampa 3.3.3): хуки
`Lampa.Listener('torrent'/'torrent_file', onlong)` добавляют пункты в
`e.menu`; экраны — `Lampa.InteractionCategory` (приём плагина collections) +
`Lampa.Api.sources.tmdb.get`; меню `Lampa.Menu.addButton`, настройки
`Lampa.SettingsApi`.

> [!IMPORTANT]
> Неофициальный хобби-проект, никакого отношения к Lampa, NNM Club и rutor
> не имеет. Парсинг трекеров опирается на гостевые страницы и может ломаться
> при редизайнах.

## Лицензия

MIT © [0x3654](https://github.com/0x3654)
