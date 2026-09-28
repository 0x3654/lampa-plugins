<div align="center">

# 🔌 Lampa Plugins

**Plugins for the [Lampa](https://lampa.mx) media app and the tracker-top backend**

[![License: MIT](https://img.shields.io/badge/License-MIT-0x3654.svg)](LICENSE)
[![Pages](https://img.shields.io/badge/install-GitHub%20Pages-222222.svg)](https://0x3654.github.io/lampa-plugins/)
[![Docker](https://img.shields.io/badge/tracker--top-ghcr.io%2F0x3654-2496ED.svg)](https://github.com/0x3654/lampa-plugins/pkgs/container/tracker-top)
[![Go](https://img.shields.io/badge/Go-1.27-00ADD8.svg)](https://go.dev)

[Install](#install) · [Torrent Send](#torrent-send) · [Top](#top) · [t.js](#tjs-bootstrap) · [tracker-top](#tracker-top) · [Development](#development) · [По-русски](#по-русски)

</div>

## Install

In Lampa: Settings → Extensions → «+», paste a URL (per device; on iPhone the
PWA is separate from the Safari tab):

| Plugin | URL |
|---|---|
| Torrent Send | `https://0x3654.github.io/lampa-plugins/transmission-send.js` |
| Top | `https://0x3654.github.io/lampa-plugins/top.js` |
| everything at once | `https://0x3654.github.io/t.js` (bootstrap, see below) |

## Torrent Send

Three items in the torrent long-press menu: **Copy magnet**, **Open magnet**
(the system `magnet:` handler — a torrent client or a helper app), and
**Add to Plex** — pushes the release into the personal "auto" feed of
[nnm-rss](https://github.com/0x3654/nnm-rss), from which transmission-rss
downloads it into the Plex library. Works with any tracker: the parser magnet
goes as-is, nnm-rss assembles the `.torrent` via DHT.

## Top

Four menu items instead of the default feed:

- **Top** — TMDB popularity: daily/weekly trending, top 14/30 days, best
  rated, new releases; variant switching with the right button.
- **Top trackers** — NNM Club + rutor release tops (video categories only),
  enriched with TMDB posters/metadata; a click opens the native film page.
- **My filter** — remembers the last applied catalog filter plus named presets
  (Lampa does not persist them itself).
- **Torrent preset** — snapshots the torrent-list filter as a named preset.

Screens assemble server-side (`/feed`) so they open instantly and survive the
"watched" filter; filter sheets mirror the native torrent filter UI.

## t.js bootstrap

A one-short-URL bootstrap for a fresh Lampa install: adds the plugins above,
applies a curated settings block once (version-gated), picks the TorrServer
link by probing, and self-heals the Apple TV platform detection glitch.
Source of truth lives in the user-site repo; the copy here mirrors it.

## tracker-top

Go service behind the "Top trackers" screen: one static binary in a scratch
image (~11 MB). Sources: NNM Club (`tracker.php?o=10`, video subtree parsed
from the page) and rutor (per-category tops). Philosophy: **films for
browsing, not a release feed** — releases of the same film collapse into one
position; quality/voice is chosen later in the Lampa parser.

```
GET /top?src=both|nnm|rut&cat=video|all&pages=1..3&minq=720|1080|2160&audio=all|dub|no_ts
GET /feed?...   server-side TMDB-top assembly with filters and watched-exclude
GET /healthz
```

In-memory cache (TTL 600 s), CORS `*`, mirror override via `NNM_BASE` /
`RUTOR_BASE`. CI builds a multi-arch image: `ghcr.io/0x3654/tracker-top`
(deps → test → build → scratch; a broken parser fails the build; non-root,
healthcheck). A public instance serves `https://top.0x3654.com`.

> [!NOTE]
> Transmission 4.1.3 removed CORS headers from its RPC, so browser/PWA
> clients cannot talk to it directly — Torrent Send routes through system
> `magnet:` handlers and native clients instead of `fetch`.

> [!IMPORTANT]
> Unofficial hobby project, no affiliation with Lampa, NNM Club or rutor.
> Tracker parsing follows guest-accessible pages and may break on redesigns.

## Development

Tests run in containers (host stays clean):

```bash
docker run --rm -v "$PWD":/src -w /src node:24-alpine node smoke-top.js
docker run --rm -v "$PWD":/src -w /src node:24-alpine node smoke-test.js
docker run --rm -v "$PWD"/tracker-top:/src -w /src golang:1.27-alpine go test ./...
```

Extension points (verified against the Lampa 3.3.3 sources): hooks
`Lampa.Listener('torrent'/'torrent_file', onlong)` push menu items into
`e.menu`; screens use `Lampa.InteractionCategory` (the collections-plugin
reception) + `Lampa.Api.sources.tmdb.get`; menus `Lampa.Menu.addButton`,
settings `Lampa.SettingsApi`.

---

# По-русски

**Torrent Send** (файл transmission-send.js): три пункта в меню долгого
нажатия на торренте — «Скопировать магнет», «Открыть магнет» и «Добавить
в Plex» (раздача уходит в ленту «авто» [nnm-rss](https://github.com/0x3654/nnm-rss),
оттуда transmission-rss скачивает её в библиотеку Plex). Настройки
Plex-кнопки — Настройки → NNM RSS.

## Установка

Таблица URL — выше, в английской части (URL одни и те же). На каждом
устройстве отдельно (PWA на iPhone — отдельно от вкладки Safari).

`nnm-auto.js` устарел (влит в Torrent Send) — на устройствах, где он был
установлен, удалите расширение; файл остаётся заглушкой с подписью.

## Top — «адекватный топ» вместо ленты

Четыре пункта в главном меню:

- **Топ** — популярное/лучшее по TMDB: тренды за день/неделю, **топ 14 и 30 дней**
  (окно дат через discover), лучшее по рейтингу, новинки 2025+. Переключение
  варианта — кнопка **вправо** на экране (или через список). Карточки и пагинация
  нативные, запросы идут через API Lampa (учитывается прокси TMDB из настроек).
- **Топ трекеров** — топ раздач NNMClub/RUTOR, только видео-категории, обогащённый
  постерами/метаданными TMDB. Клик по карточке — обычная страница фильма в Lampa
  (дальше работает transmission-send и выбор раздачи в парсере). Сортировка
  (кнопка вправо): по сидам «сейчас» или «классика» — завершённость за всё время
  (NNM `o=6`, rutor в этом режиме не участвует). Раздачи, не сматчившиеся с TMDB
  (софт, игры, сборники), не показываются.
- **Мой фильтр** — Lampa не запоминает применённый фильтр каталога; плагин ловит
  событие `activity` (`category_full` + `discover/…`) и сохраняет последний
  фильтр. Кнопка открывает список: последний применённый + именованные пресеты
  (сохранить/удалить там же).
- **Пресет торрентов** — снимает текущий фильтр списка торрентов (сортировка
  `torrents_sort` + per-карточный `torrents_filter_data`) как именованный пресет;
  применение на открытом списке перерисовывает его, вне списка — становится
  глобальным дефолтом.

Фильтры «Топа» и «Топа трекеров» вызываются кнопкой **вправо** на экране —
листом с вложенным выбором, как штатный фильтр торрентов (сортировка, качество,
озвучки, тумблеры, «Показать»); значения те же, что в Настройки → Топ.

Настройки плагина: **Настройки → Топ**:

| Параметр | Что делает |
|---|---|
| Адрес сервера топа | URL tracker-top (рабочий — `https://top.0x3654.com`) |
| «Топ» вместо главной | при запуске открывается последний вариант «Топа» вместо ленты |
| Мин. качество (трекеры) | Любое / 720p+ / 1080p+ / 2160p+ |
| Скрывать CAM/TS | камрипы и «звук с TS» вычищаются ДО дедупа: фильм только с такими раздачами исчезает целиком, с нормальной — остаётся с ней (по умолчанию вкл) |
| Только дубляж (трекеры) | в топе только фильмы, у которых есть раздача с дубляжом |

Матчинг: запрос `search/multi` по оригинальному (или русскому) названию,
жёсткий фильтр по году ±1 для фильмов (для сезонных раздач допуск больше),
порог похожести названия; дубликаты раздач схлопываются по TMDB id (остаётся
самая сидируемая).

## nnm-rss

Сервис личных RSS-лент NNM-Club живёт в отдельном репозитории:
[0x3654/nnm-rss](https://github.com/0x3654/nnm-rss). Инстанс:
**https://nnm.0x3654.com**. Torrent Send работает с ним через Bearer-API
(см. ниже «Добавить в Plex»).

## Пункты меню Torrent Send

В порядке появления (наши — после встроенных пунктов Lampa):

| Пункт | Когда показывается | Что делает |
|---|---|---|
| **Скопировать магнет** | есть магнет | магнет в буфер обмена |
| **Открыть магнет** | есть магнет | `window.location = magnet:` → системный обработчик схемы |
| **Добавить в Plex** (всегда последним) | есть магнет | раздача → лента «авто» NNM RSS → transmission-rss качает в библиотеку |

«Скачать .torrent» убран: у раздач почти никогда нет прямой http-ссылки, пункт был вечно пустой.

### «Добавить в Plex» — как работает

Кнопка отправляет раздачу одним нажатием в личную ленту «авто» сервиса
nnm-rss (NNM RSS, nnm.0x3654.com) — её забирает transmission-rss и качает
в библиотеку Plex. Работает с любым трекером: уходит магнет парсера как есть
(info-hash, трекеры, у nnm-раздач — ссылка на тему в `tr=`), `.torrent`
собирает DHT-резолвер nnm-rss.

Настройки: **Настройки → NNM RSS (nnm.0x3654.com)**:

| Параметр | Что делает |
|---|---|
| Адрес сервера | URL сервиса nnm-rss (предзаполнен `https://nnm.0x3654.com`) |
| Логин / Пароль | аккаунт на сервисе (регистрация на его главной); пароль лежит в localStorage устройства и в настройках скрыт точками |
| Проверить вход | кнопка: логинится текущим логином/паролем, статус прямо в строке (✓ зелёным / ✗ красным) |

Детали поведения:

- авторизация: JSON-логин → токен (кэшируется на пару сервер+логин), далее
  `Authorization: Bearer`; при 401 (смена пароля ротирует токен) — перелогин
  и один повтор
- магнет парсера уходит целиком, `dn=` с названием дописывается при нехватке;
  повторное добавление — noty «Уже в ленте» (дедуп на сервере по хэшу)
- разовые раздачи живут в подписках как обычные; guid стабильный —
  RSS-клиент ничего не перекачивает

Сценарии:

- **Mac**: «Открыть магнет» → приложение по умолчанию для `magnet:`
  (например, Transmission Remote GUI) → добавление на сервер. Запасной путь:
  «Скопировать магнет» → вставить в TRG.
- **iPhone**: «Открыть магнет» → приложение-обработчик magnet: (например, NASctl,
  откроет окно подтверждения). «Скопировать магнет» → вставить в NASctl.
- **Apple TV**: пункты появляются, но качать там некуда.

## Выбор приложения-обработчика magnet: на macOS

Системного UI для схем нет; переключается записью в LaunchServices
(TRG здесь для примера, bundle id `com.transgui`):

```bash
defaults write com.apple.LaunchServices/com.apple.launchservices.secure LSHandlers \
  -array-add '<dict><key>LSHandlerURLScheme</key><string>magnet</string>\
<key>LSHandlerRoleAll</key><string>com.transgui</string></dict>'
killall lsd
```

Проверить: `open 'magnet:?xt=urn:btih:0000000000000000000000000000000000000000'`.
Bundle id любого приложения: `osascript -e 'id of app "Имя"'`.

## tracker-top — сервер топов трекеров

Go, один статический бинарник (образ `scratch`, ~11 МБ). Источники:

- **NNMClub** — `tracker.php?o=10`, топ по сидам; видео-поддерево категорий
  парсится со страницы (не хардкод), cp1251
- **RUTOR** — топы по разделам: `browse/0/{cat}/0/0/2/` (сортировка по раздающим);
  видео-категории: 1/5 фильмы, 4/16 сериалы, 7 мультикация, 10 аниме, 12 доки

Философия: отдаём **фильмы для поиска, а не ленту раздач** — дубликаты раздач
одного фильма схлопываются в одну позицию (сиды суммируются, представителем
становится лучшая раздача по качеству/размеру). Качество и озвучку пользователь
выбирает позже в парсере Lampa на странице фильма; серверные фильтры оставлены
как параметры API на всякий случай.

    GET /top?src=both|nnm|rut&cat=video|all&pages=1..3&minq=720|1080|2160&audio=all|dub|no_ts
    GET /healthz

Кэш в памяти (`TTL`, 600 с), CORS `*`. Зеркала — env `NNM_BASE` / `RUTOR_BASE`.

Локально: контейнер `golang:1.27-alpine` (команда — в Development выше);
парсеры прогоняются на живых снимках страниц из `tests/`.

Образ собирает CI: `ghcr.io/0x3654/tracker-top` (multi-stage: deps → test →
build → scratch; тесты валит сборку при сломанном парсере; non-root, healthcheck).
Публичный инстанс: `https://top.0x3654.com`. В плагине сервер меняется
одной настройкой (адрес).

## Лицензия

MIT © [0x3654](https://github.com/0x3654)
