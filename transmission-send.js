/*
    Torrent Send — плагин Lampa (lampa.mx)

    Меню долгого нажатия на торренте:
      • «Скопировать магнет»  — буфер обмена (вставить в NASctl / TRG)
      • «Открыть магнет»      — системный обработчик схемы magnet:
      • «Добавить в Plex»     — раздача уезжает в ленту «авто» сервиса
                                NNM RSS (nnm.0x3654.com); transmission-rss
                                скачивает её в библиотеку Plex
                                (Настройки → NNM RSS — логин и пароль)

    Установка: Настройки → Расширения → «+» → URL этого файла.
*/

(function(){
    'use strict'

    var FLAG = '__lampa_transmission_send'

    if(window[FLAG]) return
    window[FLAG] = true

    function init(){
        var Lampa = window.Lampa

        // Lampa не умеет брать имя/описание из кода плагина — только из каталога cub.
        ;(function selfName(){
            try{
                var url   = 'https://0x3654.github.io/lampa-plugins/transmission-send.js'
                var list  = Lampa.Plugins.get()
                var named = false

                for(var i = 0; i < list.length; i++){
                    if((list[i].url || '') === url && list[i].name !== 'Torrent Send'){
                        list[i].name   = 'Torrent Send'
                        list[i].author = '@0x3654'
                        list[i].descr  = 'Магнет из долгого нажатия: скопировать или открыть. «Добавить в Plex» кладёт раздачу в ленту «авто» сервиса nnm.0x3654.com (NNM RSS) — оттуда transmission-rss скачивает её в библиотеку Plex'
                        named = true
                    }
                }

                if(named) Lampa.Plugins.save()
            }
            catch(e){}
        })()


        //---------- словарь

        Lampa.Lang.add({
            transmission_send_menu_copy:        { ru: 'Скопировать магнет',  en: 'Copy magnet' },
            transmission_send_menu_open:        { ru: 'Открыть магнет',      en: 'Open magnet' },
            transmission_send_menu_plex:        { ru: 'Добавить в Plex',     en: 'Add to Plex' },

            transmission_send_copied:           { ru: 'Магнет скопирован',   en: 'Magnet copied' },
            transmission_send_copy_fail:        { ru: 'Не удалось скопировать', en: 'Copy failed' },
            transmission_send_magnet_open:      { ru: 'Открываю магнет…',    en: 'Opening magnet…' },

            nnm_auto_added:       { ru: 'Добавлено в ленту авто',  en: 'Added to auto feed' },
            nnm_auto_exists:      { ru: 'Уже в ленте авто',        en: 'Already in auto feed' },
            nnm_auto_no_creds:    { ru: 'NNM RSS: укажите логин и пароль в настройках', en: 'NNM RSS: set login and password in settings' },
            nnm_auto_bad_creds:   { ru: 'NNM RSS: неверный логин или пароль', en: 'NNM RSS: wrong login or password' },
            nnm_auto_fail:        { ru: 'NNM RSS: не получилось добавить', en: 'NNM RSS: failed to add' },

            offline_title:     { ru: 'Офлайн',                          en: 'Offline' },
            offline_hint:      { ru: 'Обновить список',                 en: 'Refresh list' },
            offline_hint_desc: { ru: 'Раздачи встроенного движка TorrServer: качаются целиком и доступны без сети', en: 'Torrents of the embedded TorrServer engine: downloaded fully and available offline' },
            offline_loading:   { ru: 'Загружаю список…',                en: 'Loading…' },
            offline_empty:     { ru: 'Пусто: открой раздачу в плеере — она появится здесь', en: 'Empty: open a torrent in the player — it will appear here' },
            offline_ready:     { ru: 'готово',                          en: 'ready' },
            offline_fetch:     { ru: 'Докачать',                        en: 'Fetch' },
            offline_del:       { ru: 'Удалить',                         en: 'Delete' },
            offline_menu:      { ru: 'Скачать (офлайн)',                en: 'Download (offline)' },
            offline_started:   { ru: 'Скачиваю: ',                      en: 'Downloading: ' },
            offline_queued:    { ru: 'Раздача в движке, качаю',         en: 'Torrent in the engine, fetching' },
            offline_dead:      { ru: 'Встроенный движок недоступен',    en: 'Embedded engine unavailable' },

            nnm_auto_settings:        { ru: 'NNM RSS (nnm.0x3654.com)', en: 'NNM RSS (nnm.0x3654.com)' },
            nnm_auto_settings_server: { ru: 'Адрес сервера', en: 'Server address' },
            nnm_auto_settings_server_desc: {
                ru: 'Сервис NNM RSS с лентой «авто» (например, https://nnm.0x3654.com)',
                en: 'NNM RSS service with the auto feed (e.g., https://nnm.0x3654.com)'
            },
            nnm_auto_settings_login:    { ru: 'Логин', en: 'Login' },
            nnm_auto_settings_login_desc: {
                ru: 'Аккаунт на сервисе NNM RSS (регистрация на его главной)',
                en: 'Account on the NNM RSS service (register on its home page)'
            },
            nnm_auto_settings_password: { ru: 'Пароль', en: 'Password' },
            nnm_auto_settings_password_desc: {
                ru: 'Хранится в localStorage этого устройства',
                en: 'Stored in this device localStorage'
            },

            nnm_auto_settings_check: { ru: 'Проверить вход', en: 'Check login' },
            nnm_auto_check_wait:     { ru: 'Проверяю…', en: 'Checking…' },
            nnm_auto_check_ok:       { ru: '✓ Вошли как ', en: '✓ Logged in as ' },
            nnm_auto_check_bad:      { ru: '✗ Неверный логин или пароль', en: '✗ Wrong login or password' },
            nnm_auto_check_fail:     { ru: '✗ Сервер не отвечает', en: '✗ Server unreachable' }
        })

        function T(name){
            return Lampa.Lang.translate('transmission_send_' + name)
        }

        function N(name){
            return Lampa.Lang.translate('nnm_auto_' + name)
        }

        function noty(text, style){
            Lampa.Noty.show(text, style ? { style: style } : {})
        }

        function magnetOf(el){
            if(!el) return ''

            if(el.MagnetUri) return el.MagnetUri

            return /^magnet:/i.test(el.Link || '') ? el.Link : ''
        }

        function titleOf(el){
            return el ? String(el.Title || el.title || el.name || el.path_human || '') : ''
        }


        //---------- действия: копировать / открыть

        function copyMagnet(magnet){
            var ok   = function(){ noty(T('copied'), 'success') }
            var fail = function(){ noty(T('copy_fail'), 'error') }

            if(navigator.clipboard && navigator.clipboard.writeText){
                navigator.clipboard.writeText(magnet).then(ok, function(){
                    Lampa.Utils.copyTextToClipboard(magnet, ok, fail)
                })
            }
            else{
                Lampa.Utils.copyTextToClipboard(magnet, ok, fail)
            }
        }

        function openMagnet(magnet){
            noty(T('magnet_open'))
            window.location = magnet
        }


        //---------- настройки NNM RSS

        var ico_feed = '<svg viewBox="0 0 36 36" fill="none" stroke="white" stroke-width="3" stroke-linecap="round">' +
            '<circle cx="8" cy="28" r="2" fill="white" stroke="none"/>' +
            '<path d="M6 20a10 10 0 0 1 10 10"/><path d="M6 12a18 18 0 0 1 18 18"/>' +
            '</svg>'

        Lampa.SettingsApi.addComponent({
            component: 'nnm_auto',
            icon: ico_feed,
            name: N('settings')
        })

        Lampa.SettingsApi.addParam({
            component: 'nnm_auto',
            param: {
                name: 'nnm_server',
                type: 'input',
                values: 'string', // обязательный маркер для input в Lampa
                default: 'https://nnm.0x3654.com',
                placeholder: 'https://nnm.0x3654.com'
            },
            field: {
                name: N('settings_server'),
                description: N('settings_server_desc')
            }
        })

        Lampa.SettingsApi.addParam({
            component: 'nnm_auto',
            param: {
                name: 'nnm_login',
                type: 'input',
                values: 'string',
                default: '',
                placeholder: 'login'
            },
            field: {
                name: N('settings_login'),
                description: N('settings_login_desc')
            }
        })

        // строка пароля: Lampa показывает значение открытым текстом — маскируем
        // сами (onRender при каждом открытии раздела, onChange после ввода)
        var passRow = null

        function maskPass(){
            if(!passRow) return

            var v   = String(Lampa.Storage.field('nnm_password') || '')
            var val = passRow.find('.settings-param__value').eq(0)

            if(v) val.text(new Array(Math.min(v.length, 16) + 1).join('•'))
        }

        Lampa.SettingsApi.addParam({
            component: 'nnm_auto',
            param: {
                name: 'nnm_password',
                type: 'input',
                values: 'string',
                default: '',
                placeholder: '••••'
            },
            field: {
                name: N('settings_password'),
                description: N('settings_password_desc')
            },
            onRender: function(item){
                passRow = item
                maskPass()
            },
            onChange: function(){
                maskPass()
            }
        })

        // «Проверить вход»: логинимся текущим логином/паролем, статус — в строке
        var checkRow = null

        function setCheck(text, ok){
            if(!checkRow) return

            var d = checkRow.find('.settings-param__descr').eq(0)

            d.text(text)
            d[0].style.color = ok == null ? '' : (ok ? '#4ade80' : '#f87171')
        }


        //---------- API NNM RSS

        var TOKEN_KEY = 'nnm_auto_token'

        // Storage.field — значения через SettingsApi (учитывают default)
        function field(name){
            return String(Lampa.Storage.field(name) || '').trim()
        }

        function base(){
            return field('nnm_server').replace(/\/+$/, '')
        }

        function creds(){
            return {
                login:    field('nnm_login'),
                password: String(Lampa.Storage.field('nnm_password') || '')
            }
        }

        // токен кэшируем на пару сервер+логин; смена настроек его инвалидирует
        function tokenKey(){
            var c = creds()

            return TOKEN_KEY + ':' + base() + ':' + c.login
        }

        function request(method, path, body, auth, cb){
            var xhr = new XMLHttpRequest()

            xhr.open(method, base() + path, true)

            if(auth) xhr.setRequestHeader('Authorization', 'Bearer ' + auth)
            if(body != null){
                xhr.setRequestHeader('Content-Type', 'application/json')
                xhr.send(JSON.stringify(body))
            }
            else xhr.send()

            xhr.onload = function(){
                var json = null

                try{ json = JSON.parse(xhr.responseText) }
                catch(e){}

                cb(xhr.status, json)
            }
            xhr.onerror = xhr.ontimeout = function(){ cb(0, null) }
        }

        function login(cb){
            var c = creds()

            request('POST', '/api/login', { login: c.login, password: c.password }, null, function(status, json){
                if(status === 200 && json && json.token){
                    Lampa.Storage.set(tokenKey(), json.token)
                    cb(json.token)
                }
                else if(status === 401 || status === 400) cb(null, 'bad_creds')
                else cb(null, 'fail')
            })
        }

        function ensureToken(cb){
            var cached = String(Lampa.Storage.get(tokenKey(), '') || '')

            if(cached) return cb(cached, false)

            login(function(tok, err){
                if(tok) cb(tok, false)
                else cb(null, err)
            })
        }

        function runCheck(){
            var c = creds()

            if(!base() || !c.login || !c.password) return setCheck(N('no_creds'), false)

            setCheck(N('check_wait'))

            login(function(tok, err){
                if(tok) setCheck(N('check_ok') + c.login, true)
                else if(err === 'bad_creds') setCheck(N('check_bad'), false)
                else setCheck(N('check_fail'), false)
            })
        }

        Lampa.SettingsApi.addParam({
            component: 'nnm_auto',
            param: {
                name: 'nnm_auto_check',
                type: 'button'
            },
            field: {
                name: N('settings_check'),
                description: ' '
            },
            onRender: function(item){
                checkRow = item
            },
            onChange: function(){
                runCheck()
            }
        })


        //---------- «Добавить в Plex»: разовая раздача в ленту «авто»

        // btih из магнета: hex40 или base32(32) → hex40; иначе ''
        function btihOf(magnet){
            var m = /^magnet:\?[^#]*?xt=urn:btih:([a-z0-9]+)/i.exec(magnet || '')

            if(!m) return ''

            var v = m[1].toLowerCase()

            if(v.length === 40 && /^[0-9a-f]+$/.test(v)) return v

            if(v.length === 32 && /^[a-z2-7]+$/.test(v)){
                var B32 = 'abcdefghijklmnopqrstuvwxyz234567'
                var bits = 0, val = 0, out = ''

                for(var i = 0; i < v.length; i++){
                    val = (val << 5) | B32.indexOf(v[i])
                    bits += 5

                    if(bits >= 8){
                        bits -= 8
                        var d = (val >> bits) & 0xff
                        out += ('0' + d.toString(16)).slice(-2)
                    }
                }

                return out
            }

            return ''
        }

        // размер из элемента парсера: число байтов — в человекочитаемое,
        // строку («1.4 ГБ») — как есть
        function sizeOf(el){
            var v = el ? (el.Size != null ? el.Size : el.size) : null

            if(v == null || v === '') return ''
            if(typeof v !== 'string' && v > 0){
                if(v >= 1073741824) return (v / 1073741824).toFixed(1) + ' ГБ'
                if(v >= 1048576)    return (v / 1048576).toFixed(1) + ' МБ'
                return (v / 1024).toFixed(1) + ' КБ'
            }

            return String(v)
        }

        function addRelease(magnet, meta){
            var hash = btihOf(magnet)

            if(!hash) return noty(N('fail'), 'error')

            var c = creds()

            if(!base() || !c.login || !c.password) return noty(N('no_creds'))

            // исходный магнет парсера — целиком: трекеры и (у nnm-раздач)
            // ссылка на тему в tr= ; dn дописываем, если парсер не дал
            var url = /^magnet:/i.test(magnet) ? magnet : 'magnet:?xt=urn:btih:' + hash

            if(!/[?&]dn=/.test(url) && meta && meta.title)
                url += (url.indexOf('?') > -1 ? '&' : '?') + 'dn=' + encodeURIComponent(meta.title)

            var body = { url: url }

            if(meta && meta.tracker) body.tracker = meta.tracker
            if(meta && meta.size)    body.size    = meta.size

            function post(tok, retried){
                request('POST', '/api/subs', body, tok, function(status, json){
                    if(status === 200) noty(N('added'), 'success')
                    else if(status === 400 && json && /уже в подписках/.test(json.error || '')) noty(N('exists'))
                    else if((status === 401 || status === 0) && !retried){
                        // токен протух (смена пароля ротирует его) — перелогин и один повтор
                        Lampa.Storage.set(tokenKey(), '')

                        login(function(tok2, err){
                            if(tok2) post(tok2, true)
                            else if(err === 'bad_creds') noty(N('bad_creds'), 'error')
                            else noty(N('fail'), 'error')
                        })
                    }
                    else if(status === 401) noty(N('bad_creds'), 'error')
                    else noty(N('fail'), 'error')
                })
            }

            ensureToken(function(tok, err){
                if(tok) post(tok, false)
                else if(err === 'bad_creds') noty(N('bad_creds'), 'error')
                else noty(N('fail'), 'error')
            })
        }


        //---------- меню

        function wrap(prev, fn){
            return function(){
                fn()
                Lampa.Controller.toggle(prev)
            }
        }

        // порядок: копировать, открыть, «Добавить в Plex» — всегда последней
        function pushMenuItems(menu, prev, magnet, meta){
            if(!magnet) return

            menu.push({
                title: T('menu_copy'),
                onSelect: wrap(prev, function(){ copyMagnet(magnet) })
            })

            menu.push({
                title: T('menu_open'),
                onSelect: wrap(prev, function(){ openMagnet(magnet) })
            })

            menu.push({
                title: T('menu_plex'),
                onSelect: wrap(prev, function(){ addRelease(magnet, meta) })
            })
        }


        //---------- хуки

        // список раздач парсера: магнет целиком + название/трекер/размер
        Lampa.Listener.follow('torrent', function(e){
            if(e.type !== 'onlong' || !e.menu || !e.element) return

            var prev = Lampa.Controller.enabled().name
            var el   = e.element

            pushMenuItems(e.menu, prev, magnetOf(el), {
                title:   titleOf(el),
                tracker: String(el.Tracker || el.tracker || ''),
                size:    sizeOf(el)
            })
        })

        // список файлов внутри торрента (магнет восстанавливаем из info-hash)
        Lampa.Listener.follow('torrent_file', function(e){
            if(e.type !== 'onlong' || !e.menu || !e.element || !e.element.torrent_hash) return

            var prev = Lampa.Controller.enabled().name
            var el   = e.element

            pushMenuItems(e.menu, prev, 'magnet:?xt=urn:btih:' + el.torrent_hash, { title: titleOf(el) })
        })


        //---------- офлайн-библиотека (встроенный движок TorrServer)

        // видимость: только живой встроенный движок (torrserver_url_two,
        // в нашей оболочке это 127.0.0.1:8095) и включённые торренты —
        // превью/сток без движка не видят ничего
        var engineUrl = ''
        var engineAlive = false

        function engineCheck(cb){
            var two = String(Lampa.Storage.get('torrserver_url_two') || '')
            if(!two || two.indexOf('127.0.0.1') === -1){ engineAlive = false; cb(false); return }
            if(two.slice(-1) === '/') two = two.slice(0, -1)
            engineUrl = two

            var xhr = new XMLHttpRequest()
            xhr.open('GET', two + '/echo', true)
            xhr.timeout = 1500
            xhr.onload = function(){ engineAlive = xhr.status > 0; cb(engineAlive) }
            xhr.onerror = xhr.ontimeout = function(){ engineAlive = false; cb(false) }
            try{ xhr.send() }catch(e){ engineAlive = false; cb(false) }
        }

        function enginePost(json, cb){
            if(!engineUrl){ cb(null); return }
            var xhr = new XMLHttpRequest()
            xhr.open('POST', engineUrl + '/torrents', true)
            xhr.timeout = 8000
            xhr.setRequestHeader('Content-Type', 'application/json')
            xhr.onload = function(){
                var data = null
                try{ data = JSON.parse(xhr.responseText) }catch(e){}
                cb(data)
            }
            xhr.onerror = xhr.ontimeout = function(){ cb(null) }
            try{ xhr.send(JSON.stringify(json)) }catch(e){ cb(null) }
        }

        function engineList(cb){
            enginePost({action:'list'}, function(data){
                if(data && typeof data.length === 'number') cb(data)
                else if(data && data.torrents) cb(data.torrents)
                else cb([])
            })
        }

        // «скачать целиком»: добавить магнет → приоритет всех файлов
        function offlineDownload(magnet, title){
            if(!engineAlive){ Lampa.Noty.show(T('offline_dead')); return }

            Lampa.Noty.show(T('offline_started') + (title || ''))

            enginePost({action:'add', link: magnet}, function(){
                engineList(function(list){
                    var hash = ''

                    for(var i = 0; i < list.length; i++){
                        if(String(list[i].name || '') === String(title || '')){ hash = list[i].hash; break }
                    }

                    if(!hash && list.length === 1) hash = list[0].hash // только что добавленная

                    if(hash) enginePost({action:'priority', hash: hash, files: []}, function(){})
                    else Lampa.Noty.show(T('offline_queued'))
                })
            })
        }

        function fmtSize(v){
            v = Number(v) || 0
            if(v > 1024 * 1024 * 1024) return (v / 1073741824).toFixed(1) + ' ГБ'
            return Math.round(v / 1048576) + ' МБ'
        }

        // блок в настройках: список раздач движка (очередь/статус/удаление)
        var ico_offline = '<svg viewBox="0 0 36 36" fill="none" stroke="white" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M18 6 v14"/><path d="M11 15 l7 7 7-7"/><path d="M7 28 h22"/></svg>'

        function offlineSettingsInit(){
            Lampa.SettingsApi.addComponent({
                component: 'offline',
                icon: ico_offline,
                name: T('offline_title')
            })

            Lampa.SettingsApi.addParam({
                component: 'offline',
                param: {
                    name: 'offline_hint',
                    type: 'button',
                    default: ''
                },
                field: {
                    name: T('offline_hint'),
                    description: T('offline_hint_desc')
                },
                onRender: function(item){
                    item.on('hover:enter', function(){ offlineRenderList() })
                }
            })

            // страница настроек открылась — рисуем список
            Lampa.Settings.listener.follow('page', function(e){
                if(e.name !== 'offline' || !e.body) return
                offlineRenderList()
            })
        }

        var offlineTimer = null

        function offlineRenderList(){
            if(offlineTimer){ clearInterval(offlineTimer); offlineTimer = null }

            var holder = $('#offline_list')

            if(!holder.length){
                holder = $('<div id="offline_list" class="settings-param" style="padding: 12px 14px"></div>')
                $('.settings__scroll .settings-divider:last, .settings__scroll').append(holder)
            }

            holder.html('<div class="settings-param__name">' + T('offline_loading') + '</div>')

            function draw(){
                engineList(function(list){
                    if(!list.length){
                        holder.html('<div class="settings-param__name">' + T('offline_empty') + '</div>')
                        return
                    }

                    var html = ''

                    list.forEach(function(t){
                        var size  = Number(t.size) || 0
                        var ready = Number(t.preloaded) || 0
                        var pct   = size > 0 ? Math.min(100, Math.round(ready * 100 / size)) : 0

                        html += '<div class="settings-param selector offline__row" data-hash="' + t.hash + '" style="display:block;padding:8px 0">'
                        html +=   '<div class="settings-param__name" style="max-width:100%;white-space:normal">' + String(t.name || '—') + '</div>'
                        html +=   '<div style="margin:6px 0;height:4px;background:#2c2c2c;border-radius:2px;overflow:hidden"><div style="height:100%;width:' + pct + '%;background:#38b03c"></div></div>'
                        html +=   '<div style="display:flex;justify-content:space-between;align-items:center">'
                        html +=     '<span style="opacity:.55;font-size:.9em">' + pct + '% · ' + fmtSize(size) + (pct >= 100 ? ' · ' + T('offline_ready') : '') + '</span>'
                        html +=     '<span>'
                        html +=       '<button class="offline__dl" style="margin-right:12px">' + T('offline_fetch') + '</button>'
                        html +=       '<button class="offline__del">' + T('offline_del') + '</button>'
                        html +=     '</span>'
                        html +=   '</div>'
                        html += '</div>'
                    })

                    holder.html(html)

                    holder.find('.offline__dl').on('click', function(e){
                        e.stopPropagation()
                        var hash = $(this).closest('.offline__row').data('hash')
                        enginePost({action:'priority', hash: hash, files: []}, function(){ draw() })
                    })

                    holder.find('.offline__del').on('click', function(e){
                        e.stopPropagation()
                        var hash = $(this).closest('.offline__row').data('hash')
                        enginePost({action:'delete', hash: hash}, function(){ draw() })
                    })
                })
            }

            draw()
            offlineTimer = setInterval(draw, 3000)
        }

        engineCheck(function(alive){
            if(!alive) return // без движка (превью/сток) блока и пунктов нет

            offlineSettingsInit()

            // лонг-пресс: «Скачать (офлайн)» первым пунктом
            ;['torrent', 'torrent_file'].forEach(function(evt){
                Lampa.Listener.follow(evt, function(e){
                    if(e.type !== 'onlong' || !e.menu || !e.element) return

                    var el = e.element
                    var magnet = evt === 'torrent_file'
                        ? 'magnet:?xt=urn:btih:' + (el.torrent_hash || '')
                        : magnetOf(el)

                    if(!magnet) return

                    e.menu.unshift({
                        title: T('offline_menu'),
                        onSelect: wrap(Lampa.Controller.enabled().name, function(){
                            offlineDownload(magnet, titleOf(el))
                        })
                    })
                })
            })
        })


    }

    if(window.appready) init()
    else Lampa.Listener.follow('app', function(e){
        if(e.type === 'ready') init()
    })
})()
