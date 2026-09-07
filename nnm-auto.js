/*
    (устарел) — функциональность влита в Torrent Send (transmission-send.js).

    Этот файл остаётся только затем, чтобы на устройствах, где расширение
    NNM Авто было установлено, показывалась внятная подпись вместо битой.
    Ничего не делает. Удалите расширение — всё работает в Torrent Send.
*/

(function(){
    'use strict'

    var FLAG = '__lampa_nnm_auto'

    if(window[FLAG]) return
    window[FLAG] = true

    function init(){
        var Lampa = window.Lampa

        try{
            var url  = 'https://0x3654.github.io/lampa-plugins/nnm-auto.js'
            var list = Lampa.Plugins.get()
            var named = false

            for(var i = 0; i < list.length; i++){
                if((list[i].url || '') === url && list[i].name !== 'NNM Авто (устарел)'){
                    list[i].name   = 'NNM Авто (устарел)'
                    list[i].author = '@0x3654'
                    list[i].descr  = 'Влит в Torrent Send (transmission-send.js) — удалите это расширение, «Добавить в Plex» теперь там'
                    named = true
                }
            }

            if(named) Lampa.Plugins.save()
        }
        catch(e){}
    }

    if(window.appready) init()
    else Lampa.Listener.follow('app', function(e){
        if(e.type === 'ready') init()
    })
})()
