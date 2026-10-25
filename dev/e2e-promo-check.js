// проверка после вырезания ServiceEvents: промо-плагины куба не грузятся,
// в хедере нет сезонных кнопок (halloween и пр.)
const { chromium } = require('playwright')

;(async () => {
  const browser = await chromium.launch()
  const page = await (
    await browser.newContext({
      userAgent: 'Mozilla/5.0 (iPad; CPU OS 26_6 like Mac OS X)',
      viewport: { width: 1920, height: 1080 },
    })
  ).newPage()

  const pluginReqs = []
  page.on('request', r => { if (/\/plugin\//.test(r.url())) pluginReqs.push(r.url()) })

  await page.goto('https://lampa.0x3654.com', { waitUntil: 'networkidle', timeout: 60000 })
  await page.waitForTimeout(10000)

  const promoEls = await page.evaluate(() => {
    const out = []
    document.querySelectorAll('*').forEach(el => {
      const cls = String(el.className && el.className.baseVal !== undefined ? el.className.baseVal : el.className)
      if (/event|promo|halloween|black-?friday/i.test(cls)) out.push(el.tagName + '.' + cls.slice(0, 60))
    })
    return out
  })

  console.log('запросов /plugin/:', pluginReqs.length, pluginReqs.slice(0, 2))
  console.log('промо-элементов в DOM:', promoEls.length, promoEls.slice(0, 3))
  console.log(pluginReqs.length === 0 && promoEls.length === 0 ? 'PROMO CLEAN' : 'PROMO LEAK')
  await browser.close()
})()
