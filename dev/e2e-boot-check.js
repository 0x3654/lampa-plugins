// бут-проверка клона: экран должен уйти с лого и отрисовать главное меню
// (карточки/меню в DOM), без pageerror. Использование: node dev/e2e-boot-check.js [webkit|chromium]
const pw = require('playwright')
const engine = process.argv[2] === 'webkit' ? pw.webkit : pw.chromium

;(async () => {
  const browser = await engine.launch()
  const page = await (
    await browser.newContext({
      userAgent: 'Mozilla/5.0 (iPad; CPU OS 26_6 like Mac OS X)',
      viewport: { width: 1920, height: 1080 },
    })
  ).newPage()

  const errs = []
  page.on('pageerror', e => errs.push(e.message.slice(0, 120)))

  await page.goto('https://lampa.0x3654.com?cb=' + Date.now(), { waitUntil: 'domcontentloaded', timeout: 60000 })
  await page.waitForTimeout(20000)

  const r = await page
    .evaluate(() => {
      const logo = !!document.querySelector('.splash, .start, [class*=splash], [class*=logo]')
      const menu = document.querySelectorAll('.menu .menu__item, .nav .menu__item').length
      const cards = document.querySelectorAll('.card').length
      const anyText = (document.body.innerText || '').slice(0, 60).replace(/\s+/g, ' ')
      return { logo, menu, cards, anyText }
    })
    .catch(e => ({ err: e.message.slice(0, 80) }))

  console.log(engine.name(), JSON.stringify(r), 'ошибок:', errs.length, errs.slice(0, 2))
  await browser.close()
})()
