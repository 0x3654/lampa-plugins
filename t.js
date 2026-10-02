/*
    t — bootstrap-плагин Lampa (lampa.mx)

    Короткий адрес для ввода с пульта (user-site GitHub Pages, корень):
    https://0x3654.github.io/t.js

      • доустанавливает плагины (top, transmission-send) — при каждом
        старте, если плагина нет в списке; отключённые (выключенные)
        не трогает; чтобы плагин не вернулся — удалите сам t.js
      • один раз применяет настройки из CONFIG ниже (сервер топа,
        фильтры, TorrServer) и перезагружает приложение
      • повторно настройки применяются только после повышения VERSION —
        поправили CONFIG → подняли версию → при следующем запуске
        настройки перезапишутся заново (руками сделанное затрётся)
      • TorrServer: основная ссылка — сервер на ru2, дополнительная —
        встроенный (http://127.0.0.1:8090). Активная ссылка выбирается
        пробой при каждом запуске: локальный TorrServer отвечает —
        «дополнительная», нет — «основная». Сам автозапуск встроенного
        TorrServer плагином не включается (нет моста в нативное меню) —
        один раз руками: Настройки → «Настройки» (внизу списка) →
        TorrServer/автозапуск. Ручной выбор ссылки уважается: изменили
        «Использовать ссылку» сами — bootstrap больше её не трогает
        (до повышения VERSION)
      • протухшие значения переживает: нативное приложение (mx-сборка)
        синхронизирует localStorage через CUB-аккаунт и может вкатить
        в живую страницу старый облачный снапшот — мёртвые адреса
        (moro.local, micro-tracker) ПОСЛЕ наших записей. Известный
        мусор заменяем на актуальный при каждом старте и вотчдогом
        каждые 15с; чужие/свои значения не трогаем
      • самолечение платформы Apple TV: если лампа определила платформу
        неверно (окно в момент старта было не 1920×1080 — тогда пункт
        «Настройки» с нативным меню пропадает), пересчитываем признак
        после загрузки, когда размеры окна уже устоялись, и чиним

    Установка: Настройки → Расширения → «+» → URL этого файла.

    Копия для длинного URL живёт в репо lampa-plugins (t.js) —
    при правке CONFIG синхронизировать оба файла.
*/

(function(){
    'use strict'

    var FLAG = '__lampa_boot'

    if(window[FLAG]) return
    window[FLAG] = true

    // поднять после правки CONFIG — настройки применятся заново
    // v15: санитайзер мёртвых адресов + вотчдог против облачного
    // синка localStorage нативного приложения
    // v16: reload максимум раз за сессию (sessionStorage) — синк вкатывает
    // снапшот раньше маркера и reload по «boot_ver пропал» шёл циклом
    var VERSION = '16'

    // dev-контур (локальная лампа): window.TJS_DEV = true | {plugins,top}
    // true — оба адреса выводятся из адреса страницы: плагины с того же
    // origin (nginx локального контура), tracker-top на :8355 того же
    // хоста; явные значения перекрывают вывод. Прод (без TJS_DEV) —
    // значения по умолчанию ниже
    var dev = null

    try{ dev = window.TJS_DEV }catch(e){}
    if(dev === true) dev = {}

    var BASE = 'https://0x3654.github.io/lampa-plugins'
    var TOP  = 'https://top.0x3654.com'

    if(dev){
        // плагины — с того же origin; tracker-top — за nginx-прокси /topapi
        // того же origin (кросс-портовые запросы Safari режет как local
        // network: XHR падает с status 0 «нет подключения»)
        BASE = dev.plugins || window.location.origin
        TOP  = dev.top || (window.location.origin + '/topapi')
    }

    var CONFIG = {
        // status: 1 — включён; 0 — установлен выключенным (в списке есть,
        // не исполняется; включается штатно в Настройки → Расширения)
        plugins: [
            { url: BASE + '/top.js', status: 1 },
            { url: BASE + '/transmission-send.js', status: 1 },
            // etor — «разблокировщик торрентов»: включает torrents_use
            // (возвращает «Парсер»/«TorrServer» в сторовских сборках)
            { url: 'http://cub.red/plugin/etor', status: 1 },
            // прокси TMDB через cub (устойчивость к блокировкам)
            { url: 'http://cub.red/plugin/tmdb-proxy', status: 1 },
            // Plex Sync — статус просмотра Lampa ↔ аккаунт Plex
            { url: BASE + '/plex-sync.js', status: 1 }
        ],

        // вычищенные плагины: убрать из списка устройств, если остались
        // с прошлых версий бутстрапа (применяется однократно, вместе
        // с настройками — ручная установка позже не трогается);
        // v12: старые адреса репо transmission-send (репозиторий переименован
        // в lampa-plugins, Pages старого имени не редиректится)
        plugins_remove: [
            'https://0x3654.github.io/transmission-send/top.js',
            'https://0x3654.github.io/transmission-send/transmission-send.js',
            'https://0x3654.github.io/transmission-send/nnm-auto.js'
        ],

        storage: {
            // сервер «Топа · трекеров» (tracker-top; в dev — TJS_DEV.top)
            top_server_url: TOP,

            // TorrServer: основная — публичный сервер на ru2, дополнительная —
            // встроенный TorrServer приложения (Apple TV/Android/macOS).
            // Авторизация (если включится) шлётся только на активную
            // ссылку — встроенный Basic-заголовок просто игнорирует
            // секретный путь вместо авторизации; встроенный — без неё
            torrserver_url: 'https://ru2.0x3654.com/torr-31008ae1f60d63973d9ed4cb',
            torrserver_url_two: 'http://127.0.0.1:8090',

            // фильтры топа (значения — как в настройках плагина top)
            top_min_quality:  '1080',
            top_voice_1:      'Дубляж',
            top_voice_2:      'Многоголосый',
            top_trackers_sort: 'seeds',
            top_hide_watched: 'true',
            top_trackers_only: 'true',
            top_ru_titles:    'true',
            top_no_cam:       'true',
            top_hide_series:  'false',

            // «Топ» вместо главной
            top_as_home: 'true',

            // интерфейс
            background: 'false',  // показывать фон — нет
            black_style: 'true',  // чёрный стиль — да
            start_page: 'last',   // стартовая страница — последняя
            screensaver: 'false', // заставка при бездействии — нет

            // плагин Shots (cub): кнопка в карточке и кадры в плеере — нет
            shots_in_card: 'false',
            shots_in_player: 'false',

            // парсер: публичный Jackett-прокси cub (jac.red, ключ «1»)
            parser_use: 'true',
            parser_torrent_type: 'jackett',
            jackett_url: 'jac.red',
            jackett_key: '1'
        },

        // главное меню: скрыть всё, кроме Главная и Избранного
        // (наши «Топ · TMDB»/«Топ · трекеры» добавляются плагином top
        // и не попадают в скрытие); имена берём переводом ключей лампы —
        // работает при любом языке интерфейса
        menu_hide_keys: [
            'menu_feed',        // Лента
            'menu_movies',      // Фильмы
            'menu_multmovie',   // Мультфильмы
            'menu_tv',          // Сериалы
            'title_persons',    // Персоны
            'menu_catalog',     // Каталог
            'menu_filter',      // Фильтр
            'menu_relises',     // Релизы
            'menu_anime',       // Аниме
            'menu_history',     // История
            'title_subscribes', // Подписки
            'menu_timeline',    // Расписание
            'menu_torrents'     // Торренты
        ],

        // cub-плагины, подтягиваемые лампой при старте (Спорт: ключ
        // broadcast_name есть только в ихнем словаре, Shots — захардкожен;
        // редактор меню пере-применяет скрытие к кнопкам, добавленным позже)
        menu_hide_names: ['Спорт', 'Sport', 'Shots']
    }

    // dev: бутстрап устанавливает и себя — в Расширениях виден
    // «Запуск — настройка лампы», как в проде (грузится хуком index.html
    // при первом заходе, дальше исполняется как плагин)
    if(dev) CONFIG.plugins.push({ url: BASE + '/t.js', status: 1 })

    // отвечает ли локальный TorrServer (встроенный в приложение)
    // проба — GET /echo: живой эндпоинт TorrServer с CORS-заголовками.
    // Голый корень («HEAD /») встроенный в приложение сервер может
    // отдавать редиректом или без ACAO — проба падала и ссылка каждый
    // старт слетала на основную (в lampa.mx пробы нет вовсе — потому
    // там встроенный и работал)
    function probeLocal(cb){
        var tries = 3

        ;(function attempt(){
            var xhr = new XMLHttpRequest()
            var done = false

            function finish(alive){
                if(done) return
                done = true

                if(alive || !--tries) cb(alive)
                else setTimeout(attempt, 700)
            }

            xhr.open('GET', 'http://127.0.0.1:8090/echo', true)
            xhr.timeout = 1500
            xhr.onload = function(){ finish(xhr.status > 0) }
            xhr.onerror = xhr.ontimeout = function(){ finish(false) }

            try{ xhr.send() }
            catch(e){ finish(false) }
        })()
    }

    // «использовать ссылку»: два — локальный TorrServer жив, один — ru2;
    // пробуем только там, где локальный вообще бывает, на остальном — «один»
    function pickLink(cb){
        var Lampa = window.Lampa
        var local = false

        try{
            local = Lampa.Platform.is('apple_tv') || Lampa.Platform.is('android') || Lampa.Platform.macOS()
        }
        catch(e){}

        if(!local) return cb('one')

        // «липкая» двойка: если встроенный уже выбран, неудачная проба на
        // старте его не сбрасывает (сервер в приложении может отвечать
        // позже пробы) — lampa сама покажет ошибку, если он реально мёртв;
        // удачная проба всегда возвращает встроенный
        var cur = ''
        try{ cur = String(Lampa.Storage.get('torrserver_use_link') || '') }catch(e){}

        probeLocal(function(alive){
            cb(alive || cur === 'two' ? 'two' : 'one')
        })
    }

    // мёртвые значения, которые облачный синк нативного приложения может
    // вкатить обратно в живой webview ПОСЛЕ наших записей (снапшот CUB-аккаунта
    // хранит их с прошлых экспериментов): заменяем на актуальные из CONFIG.
    // Чужие значения (пользователь поставил свой сервер) не трогаем —
    // совпадение точное, в списке только заведомо дохлое
    var SANITIZE = {
        torrserver_url: ['moro.local:8090', 'http://moro.local:8090', 'https://moro.local:8090'],
        top_server_url: ['https://micro-tracker.koi-uaru.ts.net', 'https://micro-tracker.koi-uaru.ts.net/']
    }

    // читаем СЫРОЙ localStorage, не Storage.get: лампа кэширует значения
    // в памяти (readed), и вкатанный синком мусор мимо кэша невидим —
    // проверка по кэшу пропускала бы заражение. Запись — через
    // Storage.set: обновит и кэш, и localStorage
    function rawGet(key){
        var v = null

        try{ v = window.localStorage.getItem(key) }catch(e){}
        if(v === null) return ''
        try{ v = JSON.parse(v) }catch(e){}

        return String(v)
    }

    function sanitize(){
        for(var key in SANITIZE){
            var cur = rawGet(key)

            if(SANITIZE[key].indexOf(cur) > -1)
                Lampa.Storage.set(key, CONFIG.storage[key] || TOP)
        }

        // protocol: лампа строит абсолютные URL из storage 'protocol' —
        // он обязан совпадать с протоколом СТРАНИЦЫ. Протухший http на
        // https-странице ломает URL без схемы (mixed content), а https
        // на http-странице (клон для TV-приложения) шлёт TLS-хендшейки
        // в http-nginx («нет подключения»)
        try{
            var pageProto = window.location.protocol === 'https:' ? 'https' : 'http'

            if(rawGet('protocol') !== pageProto)
                Lampa.Storage.set('protocol', pageProto)
        }catch(e){}

        // платформа: синк может вернуть browser — нативные мосты и меню
        // настроек умрут; сверяемся с сырой записью (кэш Storage тут
        // слеп — Platform.get вернул бы старое значение и пропустил
        // заражение), чиним молча
        try{
            if(rawGet('platform') !== 'apple_tv' && looksLikeAppleTV()){
                Lampa.Storage.set('platform', 'apple_tv')
                Lampa.Storage.set('native', true)
            }
        }catch(e){}
    }

    // похоже ли на Apple TV по текущему состоянию (вызывается после
    // загрузки — размеры окна уже устоялись, в отличие от старта лампы);
    // «ontouchstart» отсекает настоящий iPad (у tvOS тача нет). Размер —
    // «большой и широкий», а не строго 1920×1080: WebView «подмены адреса»
    // может отдать чуть иные метрики (safe-area/скейл), и строгая проверка
    // оставляла платформу browser — без нативного меню настроек, где
    // тумблер автозапуска встроенного TorrServer
    function looksLikeAppleTV(){
        var ua = (navigator.userAgent || '').toLowerCase()

        return (ua.indexOf('ipad') > -1 || ua.indexOf('appletv') > -1 || ua.indexOf('apple tv') > -1) &&
            !('ontouchstart' in window) &&
            window.innerWidth >= 1500 && window.innerHeight >= 800
    }

    function init(){
        var Lampa = window.Lampa

        // dev: топ-сервер всегда от этого origin (хранение могло сохранить
        // прошлый вывод адреса — например, localhost:8355)
        // dev: лампа строит URL через protocol() — Storage 'protocol' у
        // свежей установки пуст и дефолтит в https://, и встроенные запросы
        // с http-страницы уходят TLS-хендшейком в наш http-nginx (400
        // «нет подключения») — форсим http
        try{
            if(dev){
                if(TOP) Lampa.Storage.set('top_server_url', TOP)
                Lampa.Storage.set('protocol', 'http')
            }
        }catch(e){}

        // имя в списке расширений (лампа берёт имя только из каталога cub)
        ;(function selfName(){
            try{
                var list = Lampa.Plugins.get()

                for(var i = 0; i < list.length; i++){
                    if((list[i].url || '').indexOf('/t.js') > -1){
                        list[i].name   = 'Запуск — настройка лампы'
                        list[i].author = '@0x3654'
                        list[i].descr  = 'Ставит плагины и применяет настройки (bootstrap)'
                    }
                }

                Lampa.Plugins.save()
            }
            catch(e){}
        })()

        // доустановить недостающие плагины
        var installed = Lampa.Plugins.get().map(function(p){ return p.url })

        CONFIG.plugins.forEach(function(plug){
            if(installed.indexOf(plug.url) === -1)
                Lampa.Plugins.add({ url: plug.url, status: plug.status, author: '@0x3654' })
        })

        // вычищенные адреса — при каждом старте, а не только после VERSION:
        // облачный синк приложения может вернуть старый список плагинов
        // (мёртвый top.js с адреса переименованного репо)
        ;(CONFIG.plugins_remove || []).forEach(function(url){
            var list = Lampa.Plugins.get()

            for(var i = 0; i < list.length; i++){
                if(list[i].url === url){ Lampa.Plugins.remove(list[i]); break }
            }
        })

        // мёртвые адреса — сразу (и дальше по вотчдогу)
        try{ sanitize() }catch(e){}

        // настройки — только при первом запуске (или после повышения VERSION)
        var MARKER = 'boot_ver'
        var LINK   = 'torrserver_use_link'
        var MEM    = 'boot_link_written'
        var reload = false

        try{
            // Storage лампы JSON-кодирует значения: get('5') вернёт ЧИСЛО 5,
            // сравнение со строкой всегда ложно — вечная перезагрузка.
            // Сравниваем строго через String (идиома top.js)
            if(String(Lampa.Storage.get(MARKER) || '') !== VERSION){
                for(var key in CONFIG.storage) Lampa.Storage.set(key, CONFIG.storage[key])

                // главное меню: нативный редактор лампы скрывает пункты
                // по ИМЕНАМ (menu_hide); переводим ключи в текущий язык
                // и добавляем сырые имена cub-плагинов
                if(CONFIG.menu_hide_keys || CONFIG.menu_hide_names){
                    var names = (CONFIG.menu_hide_keys || []).map(function(k){
                        return Lampa.Lang.translate(k)
                    }).filter(function(name){
                        return name && name.indexOf('menu_') !== 0 // непереведённый ключ — мимо
                    })

                    Lampa.Storage.set('menu_hide', names.concat(CONFIG.menu_hide_names || []))
                }

                Lampa.Storage.set(MARKER, VERSION)
                Lampa.Storage.set(MEM, '') // вернуться к авто-выбору ссылки

                reload = true
            }
        }
        catch(e){ reload = true }

        // вотчдог: облачный синк вкатывает старый снапшот и ПОСЛЕ загрузки
        // страницы (в логах приложения — «applying diff snapshot to live
        // webview» на каждом событии окна) — мёртвые адреса молча
        // заменяем обратно, без перезагрузки: лампа читает их при каждом
        // запросе, перезагрузочный цикл тут недопустим
        setInterval(function(){
            try{ sanitize() }catch(e){}
        }, 15000)

        // платформа определилась неверно (не apple_tv), а по факту это она:
        // Platform.is читает localStorage при каждом вызове, поэтому
        // достаточно записать значение — пункт нативного меню вернётся
        try{
            if(Lampa.Platform.get() !== 'apple_tv' && looksLikeAppleTV()){
                Lampa.Storage.set('platform', 'apple_tv')
                // нативный флаг лампы ставился в false при провале детекта —
                // без него нативные мосты (lampa://…) не работают
                Lampa.Storage.set('native', true)

                reload = true
            }
        }
        catch(e){}

        // активная ссылка TorrServer — пробой при каждом запуске;
        // значение, изменённое не нами, трогаем только после VERSION
        var cur     = String(Lampa.Storage.get(LINK) || '')
        var written = String(Lampa.Storage.get(MEM) || '')

        // перезагрузка — максимум РАЗ за сессию: облачный синк вкатывает
        // снапшот раньше нашего маркера, и reload по «boot_ver пропал»
        // превращался в цикл каждые 5с (значения при этом чинятся
        // молча — sanitize). Маркер в sessionStorage: синк трогает
        // только localStorage и живёт дольше одного location.reload
        function done(){
            if(!reload) return

            try{
                if(sessionStorage.getItem('__lampa_boot_rl')) return
                sessionStorage.setItem('__lampa_boot_rl', '1')
            }
            catch(e){}

            setTimeout(function(){ window.location.reload() }, 700)
        }

        if(!written || cur === written){
            pickLink(function(link){
                if(link !== cur || !written){
                    try{
                        Lampa.Storage.set(LINK, link)
                        Lampa.Storage.set(MEM, link)

                        // ссылка реально сменилась — страницу перезагрузить
                        // нужно; повторная запись того же значения — нет
                        if(link !== cur) reload = true
                    }
                    catch(e){}
                }

                done()
            })
        }
        else done()

        // свежий origin (первое включение http-адреса, чистка данных):
        // нативное приложение восстанавливает localStorage ПОСЛЕ своего
        // чека автозапуска TorrServer — ключи приезжают с опозданием,
        // TSBridge решает «флаг false» и сервер не поднимается. Если
        // автозапуск включён, а проба мертва — перезагрузка даёт нативу
        // второй чек (свой одноразовый флаг на сессию)
        setTimeout(function(){
            if(!looksLikeAppleTV()) return

            probeLocal(function(alive){
                if(alive) return

                var auto = rawGet('ts_autostart') === 'true' || rawGet('autostartMatrixOnBoot') === 'true'
                if(!auto) return

                try{
                    if(sessionStorage.getItem('__lampa_boot_ts')) return
                    sessionStorage.setItem('__lampa_boot_ts', '1')
                }
                catch(e){}

                window.location.reload()
            })
        }, 12000)
    }

    // диагностика по маркеру ?tsdiag: статус обеих ссылок TorrServer
    // одной плашкой (Noty пишется и в лог нативного приложения) —
    // проверка на TV без пульта в руках: грузим адрес с маркером
    // и читаем результат в console.txt приложения или на экране
    if(/(^|[?&])tsdiag/.test(window.location.search)){
        Lampa.Listener.follow('app', function(e){
            if(e.type !== 'ready') return

            setTimeout(function(){
                var lines = []

                var norm = function(u, cb){
                    u = String(u || '')
                    if(u && u.indexOf('://') === -1) u = 'http://' + u
                    cb(u)
                }

                var probe = function(name, u, next){
                    norm(u, function(url){
                        var xhr = new XMLHttpRequest()
                        xhr.open('GET', url.replace(/\/$/, '') + '/echo', true)
                        xhr.timeout = 4000
                        xhr.onload = function(){ lines.push(name + ' ' + xhr.status); next() }
                        xhr.onerror = xhr.ontimeout = function(){ lines.push(name + ' FAIL'); next() }
                        try{ xhr.send() }catch(e){ lines.push(name + ' FAIL'); next() }
                    })
                }

                probe('TS1:', Lampa.Storage.get('torrserver_url'), function(){
                    probe('TS2:', 'http://127.0.0.1:8090', function(){
                        Lampa.Noty.show('tsdiag · ' + lines.join(' · '), {time: 20000})
                    })
                })
            }, 25000)
        })
    }

    if(window.appready) init()
    else Lampa.Listener.follow('app', function(e){
        if(e.type === 'ready') init()
    })
})()
