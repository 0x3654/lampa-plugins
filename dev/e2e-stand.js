// e2e дев-стенда: полный бут лампы + самолечение дев-бутстрапа.
// Запуск из контейнера playwright (README → Local loop → e2e):
//   docker run --rm -v "$PWD/..":/work -w /work \
//     mcr.microsoft.com/playwright:v1.49.0-jammy node dev/e2e-stand.js
// Контейнер ходит на мак как host.docker.internal:8098.
//
// Проверяет: первый запуск (экран языка) → бутстрап ставит плагины,
// применяет настройки (menu_hide), форсит top_server_url=origin+/topapi и
// protocol=http; затем «отравляет» хранилище вчерашним состоянием
// (кросс-портовый localhost:8355 + https) и проверяет, что перезагрузка
// лечит (дев-форс на каждом старте).
const { chromium } = require('playwright')

const PAGE = process.env.STAND_URL || 'http://host.docker.internal:8098'

;(async () => {
  const browser = await chromium.launch()
  const page = await browser.newPage({ viewport: { width: 1920, height: 1080 } })

  const failed = []
  page.on('requestfailed', r => {
    const u = r.url()
    // imagetmdb/ламповые пробы — фон; ?logged= и статику гарнита рвёт
    // сама перезагрузка бутстрапа; torrserver/themoviedb — прямые заходы
    // лампы мимо наших плагинов, из контейнера им нет хода
    if (!/imagetmdb|modification\.js|personal\.lampa|black_list|\?logged=|\/img\/|torrserver\.|themoviedb|hls\.js/.test(u))
      failed.push(u.slice(0, 100) + ' :: ' + (r.failure() || {}).errorText)
  })
  page.on('pageerror', e => failed.push('PAGEERROR ' + e.message.slice(0, 120)))

  // /topapi ловим с самого начала: «Топ» — главная (top_as_home), /feed
  // уходит ещё на буте, до явного открытия экранов ниже
  const topapi = []
  const onResp = r => { if (/\/topapi\/(top|feed)/.test(r.url())) topapi.push(r.status() + ' ' + r.url().replace(/^.*\/topapi/, '/topapi').slice(0, 60)) }
  page.on('response', onResp)

  await page.goto(PAGE, { waitUntil: 'domcontentloaded' })
  await page.waitForTimeout(8000)
  // первый запуск: экран выбора языка — первый пункт «Русский»
  await page.click('.selector >> nth=0', { timeout: 5000 }).catch(() => {})
  await page.waitForTimeout(15000) // бут + применение настроек + reload

  const dump = async () => page.evaluate(() => {
    const get = k => {
      const v = localStorage.getItem(k)
      if (v === null) return null
      try { return JSON.parse(v) } catch (e) { return v }
    }
    return {
      appready: !!window.appready,
      boot_ver: get('boot_ver'),
      top_server_url: get('top_server_url'),
      protocol: get('protocol'),
      menu_hidden: (get('menu_hide') || []).length,
      plugins: (get('plugins') || []).map(p => (p.url || '').split('/').pop()),
    }
  })

  const fresh = await dump()
  console.log('fresh boot:', JSON.stringify(fresh))

  // «вчерашний» браузер: кросс-портовый топ-сервер + https-протокол
  await page.evaluate(() => {
    localStorage.setItem('top_server_url', JSON.stringify('http://localhost:8355'))
    localStorage.setItem('protocol', JSON.stringify('https'))
  })
  await page.reload({ waitUntil: 'domcontentloaded' })
  await page.waitForTimeout(15000)

  const healed = await dump()
  console.log('after poison + reload:', JSON.stringify(healed))

  // экраны Топа по-настоящему: открыть «Топ · трекеры» и «Топ · TMDB»
  // («Топ · TMDB» уже мог отработать главной на буте — push той же
  // активности лампа дедупит, поэтому /feed ловим и на буте тоже)

  const openScreen = async component => {
    // детерминированно: тот же Activity.push, что делает пункт меню
    // (UI-навигация клавиатурой зависит от того, где стоит фокус)
    try {
      await page.evaluate(c => window.Lampa.Activity.push({ url: '', title: c, component: c, page: 1 }), component)
      return true
    } catch (e) { return false }
  }

  if (await openScreen('top_trackers')) await page.waitForTimeout(12000)
  else console.log('!! экран top_trackers не открылся')

  if (await openScreen('top_screen')) await page.waitForTimeout(12000)
  else console.log('!! экран top_screen не открылся')

  page.removeListener('response', onResp)
  console.log('topapi screen requests:', topapi.length ? topapi.slice(0, 6).join('\n  ') : '(none)')

  // топ-сервер достижим из контекста страницы
  const probe = await page.evaluate(async () => {
    try {
      const r = await fetch(location.origin + '/topapi/healthz')
      return r.status
    } catch (e) { return 'FAILED ' + e.message }
  })
  console.log('probe /topapi/healthz:', probe)

  if (failed.length) console.log('failed requests:\n' + failed.join('\n'))

  // plex-sync ставится бутстрапом (v13) и исполняется: в списке расширений
  // и с зарегистрированным разделом настроек «Plex»
  const plexInstalled = fresh.plugins.includes('plex-sync.js')
  console.log('plex-sync installed:', plexInstalled ? 'yes' : 'NO')

  const ok = fresh.appready && healed.appready
    && /\/topapi$/.test(healed.top_server_url || '')
    && healed.protocol === 'http'
    && healed.menu_hidden > 0
    && probe === 200
    && topapi.some(u => u.startsWith('200 /topapi/top'))
    && topapi.some(u => u.startsWith('200 /topapi/feed'))
    && plexInstalled

  console.log(ok ? 'E2E STAND OK' : 'E2E STAND FAIL')
  await browser.close()
  process.exit(ok ? 0 : 1)
})().catch(e => { console.error('E2E ERROR', e); process.exit(2) })
