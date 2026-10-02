// e2e дев-стенда в эмуляции Apple TV: полный бут с ipad-UA и окном ровно
// 1920×1080 (детект платформы лампы), проба встроенного TorrServer на
// 127.0.0.1:8090 (контейнер torrserver публикует порт мака) и сценарий
// «облачный синк вкатил мёртвые адреса» — вотчдог t.js лечит молча.
//
// Запуск из контейнера playwright (--network host, чтобы 127.0.0.1:8090
// из страницы попадал на опубликованный порт мака):
//   docker run --rm --network host -v "$PWD":/work -w /work \
//     mcr.microsoft.com/playwright:v1.49.0-jammy node dev/e2e-atv.js
const { chromium } = require('playwright')

const PAGE = process.env.STAND_URL || 'http://localhost:8098'

// UA iPad без строки AppleTV — так определяет настоящую лампа на tvOS
const ATV_UA = 'Mozilla/5.0 (iPad; CPU OS 26_6 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko)'

const DUMP_FN = () => {
  const get = k => {
    const v = localStorage.getItem(k)
    if (v === null) return null
    try { return JSON.parse(v) } catch (e) { return v }
  }
  return {
    platform: get('platform'),
    native: get('native'),
    boot_ver: get('boot_ver'),
    use_link: get('torrserver_use_link'),
    torrserver_url: get('torrserver_url'),
    top_server_url: get('top_server_url'),
  }
}

;(async () => {
  const browser = await chromium.launch()
  const context = await browser.newContext({
    userAgent: ATV_UA,
    viewport: { width: 1920, height: 1080 },
    hasTouch: false,
  })
  const page = await context.newPage()

  await page.goto(PAGE, { waitUntil: 'domcontentloaded' })
  await page.waitForTimeout(8000)
  await page.click('.selector >> nth=0', { timeout: 5000 }).catch(() => {}) // Русский
  await page.waitForTimeout(15000) // бут + настройки + reload

  const dump = () => page.evaluate(DUMP_FN)

  const boot = await dump()
  console.log('atv boot:', JSON.stringify(boot))

  // живой локальный TorrServer (torrserver на порту мака): проба из
  // контекста страницы — то, что делает pickLink
  const probe = await page.evaluate(async () => {
    try {
      const r = await fetch('http://127.0.0.1:8090/echo')
      return r.status
    } catch (e) { return 'FAILED ' + e.message }
  })
  console.log('local torrserver /echo:', probe)

  // «облачный синк» приложения вкатывает мёртвые адреса в живой webview:
  // сырой localStorage, как это делает нативный мост mx-сборки
  await page.evaluate(() => {
    localStorage.setItem('torrserver_url', JSON.stringify('moro.local:8090'))
    localStorage.setItem('top_server_url', JSON.stringify('https://micro-tracker.koi-uaru.ts.net/'))
    localStorage.setItem('platform', JSON.stringify('browser'))
    localStorage.setItem('native', JSON.stringify(false))
  })
  await page.waitForTimeout(20000) // тик вотчдога 15с

  const healed = await dump()
  console.log('after cloud-stomp + watchdog:', JSON.stringify(healed))

  const ok = boot.platform === 'apple_tv'
    && boot.native === true
    && String(boot.boot_ver || '') !== ''
    && boot.use_link === 'two'
    && probe === 200
    && /^https:\/\/ru2/.test(boot.torrserver_url || '')
    && /^https:\/\/ru2/.test(healed.torrserver_url || '')
    && /\/topapi$/.test(healed.top_server_url || '')
    && healed.platform === 'apple_tv'

  console.log(ok ? 'E2E ATV OK' : 'E2E ATV FAIL')
  await browser.close()
  process.exit(ok ? 0 : 1)
})().catch(e => { console.error('E2E ERROR', e); process.exit(2) })
