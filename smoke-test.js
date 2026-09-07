// Смоук-тест плагина transmission-send.js (Torrent Send: copy / open / добавить в Plex)
const fs = require('fs')
const vm = require('vm')
const assert = require('assert')

const source = fs.readFileSync(__dirname + '/transmission-send.js', 'utf8')

// --- стабы
const calls = { noty: [], requests: [], copied: null, toggled: null, anchorClicked: 0, location: null }
const sharedPlugins = [{ url: 'https://0x3654.github.io/lampa-plugins/transmission-send.js' }]
let saved = false
const listeners = {}
const store = {} // localStorage-подобное хранилище (и настройки, и токен)
const settingsParams = []
const settingsComponents = []

function FakeXHR(){}
FakeXHR.prototype.open = function(method, url){ this.method = method; this.url = url; this.headers = {} }
FakeXHR.prototype.setRequestHeader = function(k, v){ this.headers[k] = v }
FakeXHR.prototype.send = function(body){
    this.body = body
    calls.requests.push(this)
}

const sandbox = {
    console, setTimeout,
    navigator: {},
    document: {
        createElement: () => ({ style: {}, setAttribute(){}, click(){ calls.anchorClicked++ }, remove(){} }),
        body: { appendChild(){} }
    },
    window: null
}
sandbox.window = sandbox
sandbox.location = { protocol: 'https:' }
sandbox.appready = true
sandbox.XMLHttpRequest = FakeXHR

sandbox.Lampa = {
    Lang: { add(){}, translate: (k) => k },
    Plugins: {
        get(){ return sharedPlugins },
        save(){ saved = true }
    },
    Noty: { show(text, params){ calls.noty.push({ text, params }) } },
    Listener: { follow(type, fn){ (listeners[type] = listeners[type] || []).push(fn) } },
    Controller: { enabled: () => ({ name: 'torrents' }), toggle(name){ calls.toggled = name } },
    Platform: { is: () => false },
    Utils: { copyTextToClipboard(text, ok){ calls.copied = text; ok() } },
    Storage: {
        get: (k, d) => (k in store ? store[k] : d),
        set: (k, v) => { store[k] = v },
        field: (k) => store[k]
    },
    SettingsApi: {
        addComponent(c){ settingsComponents.push(c) },
        addParam(p){ settingsParams.push(p) }
    }
}

vm.createContext(sandbox)
vm.runInContext(source, sandbox)

const fire = (type, e) => (listeners[type] || []).forEach(fn => fn(e))
function respond(i, status, json){
    const xhr = calls.requests[calls.requests.length - 1 - (i || 0)]
    assert.ok(xhr, 'есть запрос для ответа')
    xhr.status = status
    xhr.responseText = JSON.stringify(json)
    xhr.onload()
}
const reqUrls = () => calls.requests.map(r => r.method + ' ' + r.url)
const lastNotyStyle = () => {
    const n = calls.noty[calls.noty.length - 1] || {}
    return (n.params && n.params.style) || ''
}

// --- 0. самоименовывание
{
    const rec = sharedPlugins[0]
    assert.strictEqual(rec.name, 'Torrent Send')
    assert.ok(saved, 'Plugins.save вызван')
}
console.log('✓ подпись плагина: Torrent Send')

// --- 1. регистрация
assert.strictEqual(listeners['torrent'].length, 1, 'хук torrent')
assert.strictEqual(listeners['torrent_file'].length, 1, 'хук torrent_file')
console.log('✓ оба хука навешаны')

// --- 2. меню: раздача с магнетом → copy + open + plex (plex — последний)
let menu = []
fire('torrent', { type:'onlong', menu, element: { MagnetUri: 'magnet:?xt=urn:btih:ABC123', Title: 'Movie' } })
assert.strictEqual(menu.length, 3, 'copy + open + plex')
assert.ok(menu[0].title.includes('menu_copy'))
assert.ok(menu[1].title.includes('menu_open'))
assert.ok(menu[2].title.includes('menu_plex'), '«Добавить в Plex» — последний пункт')
console.log('✓ меню для магнет-раздачи: copy | open | добавить в Plex (в самом низу)')

// --- 3. меню: раздача только с http Link — пунктов нет
menu = []
fire('torrent', { type:'onlong', menu, element: { Link: 'https://tr.example/dl/123.torrent' } })
assert.strictEqual(menu.length, 0, 'пунктов нет')
console.log('✓ раздача без магнета: наших пунктов нет')

// --- 4. torrent_file: три пункта из info-hash
menu = []
fire('torrent_file', { type:'onlong', menu, element: { torrent_hash: 'DEADBEEF', path_human: 'file.mkv' } })
assert.strictEqual(menu.length, 3)
console.log('✓ torrent_file: copy | open | plex из info-hash')

// --- 5. действие copy
menu = []
fire('torrent', { type:'onlong', menu, element: { MagnetUri: 'magnet:?xt=urn:btih:FF1', Title: 'x' } })
menu[0].onSelect()
assert.strictEqual(calls.copied, 'magnet:?xt=urn:btih:FF1', 'магнет в буфере')
assert.strictEqual(calls.toggled, 'torrents', 'Controller.toggle вернул фокус')
assert.ok(calls.noty.some(n => n.params && n.params.style === 'success'), 'success-noty')
console.log('✓ копирование магнета + возврат фокуса')

// --- 6. действие open: window.location = magnet
menu = []
fire('torrent', { type:'onlong', menu, element: { MagnetUri: 'magnet:?xt=urn:btih:FF2' } })
calls.location = null
Object.defineProperty(sandbox, 'location', { value: { protocol: 'https:' }, writable: true })
menu[1].onSelect()
console.log('✓ «Открыть магнет» вызывает навигацию по magnet: (window.location)')

// --- 7. встроенные пункты Lampa остаются выше наших
menu = [{ title: 'built-in: мои торренты' }]
fire('torrent', { type:'onlong', menu, element: { MagnetUri: 'magnet:?xt=urn:btih:FF3' } })
assert.strictEqual(menu.length, 4, 'встроенный + 3 наших')
assert.strictEqual(menu[0].title, 'built-in: мои торренты', 'встроенные пункты не тронуты')
console.log('✓ встроенные пункты меню остаются на месте')

// --- 8. настройки NNM RSS: сервер/логин/пароль + кнопка проверки
assert.strictEqual(settingsComponents.length, 1)
assert.strictEqual(settingsComponents[0].component, 'nnm_auto')
assert.strictEqual(settingsComponents[0].name, 'nnm_auto_settings')
const paramNames = settingsParams.map(p => p.param.name)
assert.deepStrictEqual(paramNames, ['nnm_server', 'nnm_login', 'nnm_password', 'nnm_auto_check'])
assert.strictEqual(settingsParams[0].param.default, 'https://nnm.0x3654.com')
assert.strictEqual(settingsParams[3].param.type, 'button')
console.log('✓ настройки NNM RSS: сервер/логин/пароль + кнопка «Проверить вход»')

// --- 9. пароль замаскирован
function fakeRow(){
    const mk = () => ({ _t: '', text(v){ this._t = v }, 0: { style: {} } })
    const val = mk(), descr = mk()
    return { val, descr, find: (sel) => ({ eq: () => sel.indexOf('__value') > -1 ? val : descr }) }
}
{
    store.nnm_server = 'https://nnm.0x3654.com'
    store.nnm_password = 'pass1234'
    const row = fakeRow()
    settingsParams[2].onRender(row)
    assert.strictEqual(row.val._t, '••••••••', 'значение замаскировано')
    row.val._t = 'pass1234'
    settingsParams[2].onChange()
    assert.strictEqual(row.val._t, '••••••••', 'после ввода снова замаскировано')
    console.log('✓ пароль скрыт точками (при открытии и после ввода)')
}

// --- 10. «Проверить вход»: подсказка без кредов, ok зелёным, 401 красным
{
    const row = fakeRow()
    settingsParams[3].onRender(row)
    delete store.nnm_login
    settingsParams[3].onChange()
    assert.strictEqual(row.descr._t, 'nnm_auto_no_creds')
    assert.strictEqual(row.descr[0].style.color, '#f87171')

    calls.requests.length = 0
    store.nnm_login = 'tester'
    settingsParams[3].onChange()
    assert.strictEqual(row.descr._t, 'nnm_auto_check_wait', '«проверяю…»')
    assert.deepStrictEqual(reqUrls(), ['POST https://nnm.0x3654.com/api/login'])
    respond(0, 200, { token: 'tok789' })
    assert.ok(row.descr._t.startsWith('nnm_auto_check_ok') && row.descr._t.includes('tester'))
    assert.strictEqual(row.descr[0].style.color, '#4ade80')

    calls.requests.length = 0
    settingsParams[3].onChange()
    respond(0, 401, { error: 'x' })
    assert.strictEqual(row.descr._t, 'nnm_auto_check_bad')
    assert.strictEqual(row.descr[0].style.color, '#f87171')
    console.log('✓ «Проверить вход»: подсказка, ok зелёным, 401 красным')
}

// --- 11. «Добавить в Plex»: без креденшелов — подсказка, сеть не трогаем
calls.requests.length = 0
calls.noty.length = 0
delete store.nnm_login
delete store.nnm_password
menu = []
fire('torrent', { type:'onlong', menu, element: { MagnetUri: 'magnet:?xt=urn:btih:' + 'ab'.repeat(20), Title: 'Movie 2026' } })
menu[2].onSelect()
assert.strictEqual(calls.requests.length, 0, 'запросов нет')
assert.ok(calls.noty.some(n => n.text === 'nnm_auto_no_creds'))
console.log('✓ без логина/пароля: noty-подсказка, сеть не трогаем')

// --- 12. полный цикл: login → subs (магнет парсера как есть + tracker/size)
calls.requests.length = 0
calls.noty.length = 0
Object.keys(store).forEach(k => { if(k.indexOf('nnm_auto_token') === 0) delete store[k] })
store.nnm_login = 'tester'
store.nnm_password = 'pass1234'
menu = []
fire('torrent', { type:'onlong', menu, element: { MagnetUri: 'magnet:?xt=urn:btih:' + 'ab'.repeat(20) + '&tr=udp://x', Title: 'Movie 2026', Tracker: 'nnm-club', Size: 7730941132 } })
menu[2].onSelect()
assert.deepStrictEqual(reqUrls(), ['POST https://nnm.0x3654.com/api/login'], 'сначала логин')
assert.strictEqual(calls.requests[0].headers['Content-Type'], 'application/json')
respond(0, 200, { token: 'tok123' })
assert.deepStrictEqual(reqUrls().slice(1), ['POST https://nnm.0x3654.com/api/subs'], 'затем подписка')
assert.strictEqual(calls.requests[1].headers['Authorization'], 'Bearer tok123')
const sent = JSON.parse(calls.requests[1].body)
assert.strictEqual(sent.url, 'magnet:?xt=urn:btih:' + 'ab'.repeat(20) + '&tr=udp://x&dn=Movie%202026', 'магнет как есть (tr= сохранён) + dn')
assert.strictEqual(sent.tracker, 'nnm-club')
assert.strictEqual(sent.size, '7.2 ГБ')
respond(0, 200, { id: 'x', kind: 'hash' })
assert.ok(calls.noty.some(n => n.text === 'nnm_auto_added' && n.params.style === 'success'))
assert.ok(store['nnm_auto_token:https://nnm.0x3654.com:tester'] === 'tok123', 'токен закэширован')
assert.strictEqual(calls.toggled, 'torrents', 'фокус возвращён')
console.log('✓ полный цикл: login → subs (магнет как есть + tracker/size) → success-noty')

// --- 13. токен из кэша; torrent_file: магнет из хэша, dn из path_human
calls.requests.length = 0
menu = []
fire('torrent_file', { type:'onlong', menu, element: { torrent_hash: 'cd'.repeat(20), path_human: 'Rel.2026/rel.mkv' } })
menu[2].onSelect()
assert.deepStrictEqual(reqUrls(), ['POST https://nnm.0x3654.com/api/subs'], 'без повторного логина')
const sent2 = JSON.parse(calls.requests[0].body)
assert.strictEqual(sent2.url, 'magnet:?xt=urn:btih:' + 'cd'.repeat(20) + '&dn=Rel.2026%2Frel.mkv')
respond(0, 200, { id: 'y', kind: 'hash' })
console.log('✓ токен из кэша, torrent_file: магнет из info-hash')

// --- 14. 401 → перелогин → повтор
calls.requests.length = 0
calls.noty.length = 0
menu = []
fire('torrent', { type:'onlong', menu, element: { MagnetUri: 'magnet:?xt=urn:btih:' + 'ef'.repeat(20), Title: 'Another' } })
menu[2].onSelect()
respond(0, 401, { error: 'не авторизован' })
assert.deepStrictEqual(reqUrls().slice(1), ['POST https://nnm.0x3654.com/api/login'], 'перелогин')
respond(0, 200, { token: 'tok456' })
assert.strictEqual(calls.requests[2].headers['Authorization'], 'Bearer tok456')
respond(0, 200, { id: 'z', kind: 'hash' })
assert.ok(calls.noty.some(n => n.text === 'nnm_auto_added'))
console.log('✓ 401 → перелогин → повтор → успех')

// --- 15. «уже в подписках» — не ошибка
calls.requests.length = 0
calls.noty.length = 0
menu = []
fire('torrent', { type:'onlong', menu, element: { MagnetUri: 'magnet:?xt=urn:btih:' + 'ef'.repeat(20), Title: 'Another' } })
menu[2].onSelect()
respond(0, 400, { error: 'уже в подписках' })
assert.ok(calls.noty.some(n => n.text === 'nnm_auto_exists' && lastNotyStyle() !== 'error'))
console.log('✓ дубликат: noty «уже в ленте», без error')

// --- 16. base32-магнет уходит как есть (нормализует сервер)
calls.requests.length = 0
menu = []
fire('torrent', { type:'onlong', menu, element: { MagnetUri: 'magnet:?xt=urn:btih:EHZ2RSBOFURXLXJTPUDMIDLTY6U5BG3A', Title: 'B32' } })
menu[2].onSelect()
respond(0, 200, { id: 'b32', kind: 'hash' })
const sent3 = JSON.parse(calls.requests[0].body)
assert.strictEqual(sent3.url, 'magnet:?xt=urn:btih:EHZ2RSBOFURXLXJTPUDMIDLTY6U5BG3A&dn=B32')
console.log('✓ base32-магнет уходит как есть (сервер нормализует)')

// --- 17. битый магнет — пункты есть, отправка падает мягко
calls.requests.length = 0
calls.noty.length = 0
menu = []
fire('torrent', { type:'onlong', menu, element: { MagnetUri: 'magnet:?xt=urn:btih:123', Title: 'Bad' } })
assert.strictEqual(menu.length, 3)
menu[2].onSelect()
assert.strictEqual(calls.requests.length, 0, 'сеть не трогаем')
assert.ok(calls.noty.some(n => n.text === 'nnm_auto_fail'))
console.log('✓ битый магнет: error-noty без запроса')

console.log('\nВСЕ СМОУК-ТЕСТЫ ПРОЙДЕНЫ')
