// e2e ПРОД-клона (https://lampa.0x3654.com): бут в эмуляции Apple TV
// и проверка, что чужой слежки нет — срез лампы lampainit работает.
// В отличие от e2e-stand.js (дев-контур localhost) гоняется против
// прода, потому что XHR-патч живёт в lampainit.js клона, которого в
// дев-вебруте нет.
//
//   docker run --rm -v "$PWD":/work -w /work \
//     mcr.microsoft.com/playwright:v1.49.0-jammy bash -c \
//     "npm install --no-save --silent playwright@1.49.0; node dev/e2e-clone.js"
const { chromium } = require('playwright')

const PAGE = process.env.CLONE_URL || 'https://lampa.0x3654.com'
const ATV_UA = 'Mozilla/5.0 (iPad; CPU OS 26_6 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko)'

;(async () => {
  const browser = await chromium.launch()
  const ctx = await browser.newContext({
    userAgent: ATV_UA,
    viewport: { width: 1920, height: 1080 },
    hasTouch: false,
  })
  const page = await ctx.newPage()

  const bad = []
  const leaks = []
  let referrerPolicy = '(none)'

  page.on('response', r => {
    if (r.url() === PAGE + '/') referrerPolicy = r.headers()['referrer-policy'] || '(none)'
  })
  page.on('request', r => {
    const u = r.url()
    // аналитика / реклама / гео cub
    if (/\/api\/(ad\/get|metric)\//.test(u)) bad.push(u.slice(0, 90))
    if (/^https?:\/\/geo\./.test(u)) bad.push(u.slice(0, 90))
    // Shots и tsarea (реклама аренды TorrServer в настройках сервера)
    // вырезаны целиком — скрипты не должны грузиться даже about:blank'ом
    if (/\/plugin\/(shots|tsarea)/.test(u)) bad.push(u.slice(0, 90))
    // их включатель торрентов больше не ставится
    if (/cub\.red\/plugin\/etor/.test(u)) bad.push(u.slice(0, 90))
    // метки чужим хостам
    if (u.indexOf('lampa.0x3654.com') === -1 && /[?&](logged|email)=/.test(u))
      leaks.push(u.slice(0, 90))
  })

  await page.goto(PAGE, { waitUntil: 'domcontentloaded' })
  await page.waitForTimeout(8000)
  await page.click('.selector >> nth=0', { timeout: 5000 }).catch(() => {}) // Русский
  await page.waitForTimeout(20000) // бут, загрузка плагинов, прогулка по экранам

  const state = await page.evaluate(() => {
    const g = k => { try { return JSON.parse(localStorage.getItem(k)) } catch (e) { return localStorage.getItem(k) } }
    return {
      appready: !!window.appready,
      boot_ver: g('boot_ver'),
      torrents_use: !!(window.lampa_settings && window.lampa_settings.torrents_use),
      plugins: (g('plugins') || []).map(p => (p.url || '').split('/').pop()),
    }
  })

  console.log('boot:', JSON.stringify(state))
  console.log('referrer-policy:', referrerPolicy)
  console.log('foreign analytics/ad/geo:', bad.length === 0 ? 'NONE' : bad.join('\n  '))
  console.log('logged/email leaks:', leaks.length === 0 ? 'NONE' : leaks.join('\n  '))

  const ok = state.appready
    && String(state.boot_ver || '') !== ''
    && state.torrents_use
    && referrerPolicy === 'no-referrer'
    && bad.length === 0
    && leaks.length === 0
    && !state.plugins.includes('etor')

  console.log(ok ? 'E2E CLONE OK' : 'E2E CLONE FAIL')
  await browser.close()
  process.exit(ok ? 0 : 1)
})().catch(e => { console.error('E2E ERROR', e); process.exit(2) })
