#!/usr/bin/env python3
# патчи вебрута клона (идемпотентны, запускаются после git pull):
#   1) hasPremium → безусловно 1: куб-статусы (Смотрю/Просмотрено/…) не лочатся,
#      статусы синкаются plex-sync'ом в marks, куб-аккаунт не нужен
#   2) ServiceEvents → выход до загрузки сезонных промо-плагинов куба
#      (halloween/black-friday/snow/new-year/womens-day рисуют мигающую рекламу)
# Каждый патч: точный literal/regex, счётчик вхождений строго 1, иначе FAIL —
# апстрим поменял код, патч требует ревизии (молча ломать бандл нельзя)
import re
import sys

PATH = sys.argv[1] if len(sys.argv) > 1 else "/server/lampaweb/webroot/app.min.js"

s = open(PATH, encoding="utf-8").read()
patched = []

# 1) премиум-замки
old = "return user.id ? Utils$1.countDays(Date.now(), user.premium) : 0;"
if old in s:
    assert s.count(old) == 1, "hasPremium: несколько вхождений"
    s = s.replace(old, "return 1;")
    patched.append("hasPremium → 1")
elif "if (Storage.get('developer_nopremium', 'false')) return 0;      return 1;" in s:
    patched.append("hasPremium уже применён")
else:
    sys.exit("FAIL: hasPremium строка не найдена — апстрим изменил код")

# 2) сезонные промо: гейт ServiceEvents заменяем выходом, ИМЯ функции
# сохраняем capture-группой (минификатор даёт init$N — рвётся только вместе
# с ссылкой ServiceEvents.init = init$N, что вешает бут на лого)
pat = re.compile(r"(function init\$\d+\(\) \{\s*)if \(!window\.lampa_settings\.services\) return;")
m = pat.search(s)
if m:
    s = pat.sub(r"\1return;", s, count=1)
    patched.append("ServiceEvents → return")
elif re.search(r"function init\$\d+\(\) \{\s*return;\s*events\.forEach", s):
    patched.append("ServiceEvents уже применён")
else:
    sys.exit("FAIL: ServiceEvents гейт не найден — апстрим изменил код")

open(PATH, "w", encoding="utf-8").write(s)
print("OK:", ", ".join(patched))
