-- Hub 控制条。每条 ASS 事件必须单独换行，否则后续 \pos 会全部叠到左上角。
local overlay = mp.create_osd_overlay("ass-events")
local chrome_until = 0
local dragging = false
local dragging_vol = false
local last_up = 0
local pending_pause = nil
local painted_visible = true
local pressed_id = nil

local COLOR_BG = "&H0A0807&"
local COLOR_TEXT = "&HEBF1F2&"
local COLOR_MUTED = "&HBFC6C4&"
local COLOR_DIM = "&H8E9296&"
local COLOR_ACCENT = "&H99D334&"
local PAD = 28
local BTN = 32
local SPEED_W = 42
local VOL_W = 76
local SPEEDS = { 0.75, 1, 1.25, 1.5, 2 }

local function now()
    return mp.get_time()
end

local function show_chrome()
    chrome_until = now() + 2.4
end

local function chrome_visible(paused)
    return paused or now() < chrome_until or dragging or dragging_vol
end

local function ass_escape(text)
    return (text:gsub("\\", "\\\\"):gsub("{", "\\{"):gsub("}", "\\}"))
end

local function clock(seconds)
    seconds = math.max(0, math.floor(seconds or 0))
    local h = math.floor(seconds / 3600)
    local m = math.floor(seconds % 3600 / 60)
    local s = seconds % 60
    if h > 0 then
        return string.format("%d:%02d:%02d", h, m, s)
    end
    return string.format("%d:%02d", m, s)
end

local function osd_size()
    local dim = mp.get_property_native("osd-dimensions")
    if dim and (dim.w or 0) >= 32 and (dim.h or 0) >= 32 then
        return dim.w, dim.h
    end
    local w, h = mp.get_osd_size()
    return w, h
end

local function push(lines, event)
    if event and event ~= "" then
        lines[#lines + 1] = event
    end
end

local function draw_rect(x, y, w, h, color, alpha)
    if w <= 0 or h <= 0 then
        return ""
    end
    return string.format(
        "{\\an7\\bord0\\shad0\\p1\\c%s\\alpha&H%s&\\pos(%.1f,%.1f)}m 0 0 l %.1f 0 l %.1f %.1f l 0 %.1f{\\p0}",
        color, alpha, x, y, w, w, h, h
    )
end

local function draw_circle(cx, cy, r, color, alpha)
    if r <= 0 then return "" end
    local k = r * 0.55228
    return string.format(
        "{\\an7\\bord0\\shad0\\p1\\c%s\\alpha&H%s&\\pos(%.1f,%.1f)}"
            .. "m %.1f 0 "
            .. "b %.1f %.1f %.1f %.1f 0 %.1f "
            .. "b %.1f %.1f %.1f %.1f %.1f 0 "
            .. "b %.1f %.1f %.1f %.1f 0 %.1f "
            .. "b %.1f %.1f %.1f %.1f %.1f 0{\\p0}",
        color, alpha, cx, cy,
        r,
        r, k, k, r, r,
        -k, r, -r, r, -r,
        -r, -k, -k, -r, -r,
        k, -r, r, -r, r
    )
end

local function draw_gradient(lines, x, y, w, h, from_top, steps, base_alpha)
    local step_h = h / steps
    for i = 0, steps - 1 do
        local t = i / math.max(1, steps - 1)
        local a = math.floor(base_alpha * (1 - t) * (1 - t))
        if a > 0 then
            local cur_y = from_top and (y + i * step_h) or (y + h - (i + 1) * step_h)
            push(lines, draw_rect(x, cur_y, w, step_h + 0.6, COLOR_BG, string.format("%02X", 255 - a)))
        end
    end
end

local function hit(box, x, y)
    return x >= box.x and x <= box.x + box.w and y >= box.y and y <= box.y + box.h
end

local function layout(w, h)
    local row_y = h - 42
    local prev = { id = "prev", x = PAD, y = row_y, w = BTN, h = BTN }
    local play = { id = "play", x = prev.x + BTN + 6, y = row_y, w = BTN, h = BTN }
    local nxt = { id = "next", x = play.x + BTN + 6, y = row_y, w = BTN, h = BTN }
    local fs = { id = "fs", x = w - PAD - BTN, y = row_y, w = BTN, h = BTN }
    local sub = { id = "sub", x = fs.x - 6 - BTN, y = row_y, w = BTN, h = BTN }
    local audio = { id = "audio", x = sub.x - 6 - BTN, y = row_y, w = BTN, h = BTN }
    local speed = { id = "speed", x = audio.x - 6 - SPEED_W, y = row_y, w = SPEED_W, h = BTN }
    local vol = { id = "vol", x = speed.x - 6 - BTN, y = row_y, w = BTN, h = BTN }
    local time_x = nxt.x + BTN + 12
    local show_volbar = (vol.x - 8 - VOL_W) > (time_x + 130)
    local volbar = {
        id = "volbar",
        x = show_volbar and (vol.x - 8 - VOL_W) or vol.x,
        y = row_y + 8,
        w = show_volbar and VOL_W or 0,
        h = 16,
    }
    local volbar_hit = {
        id = "volbar",
        x = volbar.x,
        y = row_y,
        w = volbar.w,
        h = BTN,
    }
    local seek = {
        id = "seek",
        x = PAD,
        y = h - 68,
        w = w - PAD * 2,
        h = 20,
    }
    return {
        prev = prev,
        play = play,
        nxt = nxt,
        vol = vol,
        volbar = volbar,
        volbar_hit = volbar_hit,
        speed = speed,
        audio = audio,
        sub = sub,
        fs = fs,
        seek = seek,
        buttons = { prev, play, nxt, vol, speed, audio, sub, fs },
    }
end

local function button_at(geom, x, y)
    for i = 1, #geom.buttons do
        local box = geom.buttons[i]
        if hit(box, x, y) then
            return box.id
        end
    end
    return nil
end

local function draw_icon_play(cx, cy, color)
    return string.format(
        "{\\an7\\bord0\\shad0\\p1\\c%s\\pos(%.1f,%.1f)}m 0 0 l 12 7 l 0 14{\\p0}",
        color, cx - 5, cy - 7
    )
end

local function draw_icon_pause(lines, cx, cy, color)
    push(lines, draw_rect(cx - 5.2, cy - 6.5, 3.2, 13, color, "00"))
    push(lines, draw_rect(cx + 2.0, cy - 6.5, 3.2, 13, color, "00"))
end

local function draw_icon_prev(lines, cx, cy, color)
    push(lines, draw_rect(cx - 7.5, cy - 5.5, 2.2, 11, color, "00"))
    push(lines, string.format(
        "{\\an7\\bord0\\shad0\\p1\\c%s\\pos(%.1f,%.1f)}m 11 0 l 0 5.5 l 11 11{\\p0}",
        color, cx - 4.5, cy - 5.5
    ))
end

local function draw_icon_next(lines, cx, cy, color)
    push(lines, string.format(
        "{\\an7\\bord0\\shad0\\p1\\c%s\\pos(%.1f,%.1f)}m 0 0 l 11 5.5 l 0 11{\\p0}",
        color, cx - 6.5, cy - 5.5
    ))
    push(lines, draw_rect(cx + 5.3, cy - 5.5, 2.2, 11, color, "00"))
end

local function draw_icon_vol(lines, cx, cy, muted, color)
    -- 扬声器底座与锥体
    push(lines, draw_rect(cx - 8.5, cy - 3, 3.5, 6, color, "00"))
    push(lines, string.format(
        "{\\an7\\bord0\\shad0\\p1\\c%s\\pos(%.1f,%.1f)}m 0 3 l 4 0 l 4 12 l 0 9{\\p0}",
        color, cx - 5, cy - 6
    ))
    if muted then
        -- 静音斜杠
        push(lines, string.format(
            "{\\an7\\bord0\\shad0\\p1\\c%s\\pos(%.1f,%.1f)}m 0 1.5 l 1.5 0 l 6.5 5 l 5 6.5{\\p0}",
            color, cx + 1.5, cy - 3.2
        ))
        push(lines, string.format(
            "{\\an7\\bord0\\shad0\\p1\\c%s\\pos(%.1f,%.1f)}m 5 0 l 6.5 1.5 l 1.5 6.5 l 0 5{\\p0}",
            color, cx + 1.5, cy - 3.2
        ))
        return
    end
    -- 声波弧线（精细闭合多边形）
    push(lines, string.format(
        "{\\an7\\bord0\\shad0\\p1\\c%s\\pos(%.1f,%.1f)}m 0 1 l 1.4 0 b 3.4 2.2 3.4 4.8 1.4 7 l 0 6 b 1.8 4.2 1.8 2.8 0 1{\\p0}",
        color, cx + 1.2, cy - 3.5
    ))
    push(lines, string.format(
        "{\\an7\\bord0\\shad0\\p1\\c%s\\pos(%.1f,%.1f)}m 0 1.4 l 1.4 0 b 4.2 3.2 4.2 7.8 1.4 11 l 0 9.6 b 2.4 7.0 2.4 4.0 0 1.4{\\p0}",
        color, cx + 3.8, cy - 5.5
    ))
end

local function draw_icon_cc(cx, cy, on, color)
    local label = on and "CC" or "Cc"
    return string.format("{\\an5\\bord0\\shad0\\fs13\\b1\\c%s\\pos(%.1f,%.1f)}%s", color, cx, cy, label)
end

local function draw_icon_audio(cx, cy, color)
    return string.format("{\\an5\\bord0\\shad0\\fs14\\b1\\c%s\\pos(%.1f,%.1f)}A", color, cx, cy)
end

local function draw_icon_fs(lines, cx, cy, full, color)
    if full then
        -- 全屏状态：向内收敛的 4 个直角
        push(lines, draw_rect(cx - 7, cy - 1.8, 5.5, 1.8, color, "00"))
        push(lines, draw_rect(cx - 3.3, cy - 5.5, 1.8, 3.7, color, "00"))
        push(lines, draw_rect(cx + 1.5, cy - 1.8, 5.5, 1.8, color, "00"))
        push(lines, draw_rect(cx + 1.5, cy - 5.5, 1.8, 3.7, color, "00"))
        push(lines, draw_rect(cx - 7, cy, 5.5, 1.8, color, "00"))
        push(lines, draw_rect(cx - 3.3, cy + 1.8, 1.8, 3.7, color, "00"))
        push(lines, draw_rect(cx + 1.5, cy, 5.5, 1.8, color, "00"))
        push(lines, draw_rect(cx + 1.5, cy + 1.8, 1.8, 3.7, color, "00"))
        return
    end
    -- 窗口状态：向外展开的 4 个直角
    push(lines, draw_rect(cx - 7, cy - 7, 5.5, 1.8, color, "00"))
    push(lines, draw_rect(cx - 7, cy - 5.2, 1.8, 3.7, color, "00"))
    push(lines, draw_rect(cx + 1.5, cy - 7, 5.5, 1.8, color, "00"))
    push(lines, draw_rect(cx + 5.2, cy - 5.2, 1.8, 3.7, color, "00"))
    push(lines, draw_rect(cx - 7, cy + 5.2, 5.5, 1.8, color, "00"))
    push(lines, draw_rect(cx - 7, cy + 1.5, 1.8, 3.7, color, "00"))
    push(lines, draw_rect(cx + 1.5, cy + 5.2, 5.5, 1.8, color, "00"))
    push(lines, draw_rect(cx + 5.2, cy + 1.5, 1.8, 3.7, color, "00"))
end

local function seek_to_ratio(ratio)
    mp.commandv("seek", math.min(1, math.max(0, ratio)) * 100, "absolute-percent")
end

local function volume_from_x(geom, x)
    local bar = geom.volbar
    if bar.w <= 0 then
        return
    end
    local ratio = math.min(1, math.max(0, (x - bar.x) / bar.w))
    mp.set_property_bool("mute", false)
    mp.set_property_number("volume", ratio * 100)
end

local function cycle_fullscreen()
    mp.commandv("cycle", "fullscreen")
end

local function cycle_speed()
    local cur = mp.get_property_number("speed") or 1
    local idx = 1
    for i = 1, #SPEEDS do
        if math.abs(cur - SPEEDS[i]) < 0.06 then
            idx = i
            break
        end
    end
    mp.set_property_number("speed", SPEEDS[(idx % #SPEEDS) + 1])
end

local function speed_label(speed)
    if math.abs((speed or 1) - 1) < 0.04 then
        return "1×"
    end
    local text = string.format("%.2f", speed)
    text = text:gsub("0+$", ""):gsub("%.$", "")
    return text .. "×"
end

local function track_off(value)
    return value == nil or value == false or value == "no" or value == "false"
end

local function track_label(kind)
    local prop = kind == "audio" and "aid" or "sid"
    local prefix = kind == "audio" and "current-tracks/audio/" or "current-tracks/subtitle/"
    if track_off(mp.get_property(prop)) then
        return kind == "audio" and "无音轨" or "字幕关"
    end
    local title = mp.get_property(prefix .. "title")
    local lang = mp.get_property(prefix .. "lang")
    if title and title ~= "" then
        return title
    end
    if lang and lang ~= "" then
        return lang
    end
    local id = mp.get_property(prop)
    if kind == "audio" then
        return "音轨 " .. tostring(id)
    end
    return "字幕 " .. tostring(id)
end

-- 写到临时文件，Hub 轮询 native_status 时读走。mpv 自己没有 Hub 播放队列。
local function request_skip(which)
    local dir = os.getenv("TMPDIR") or os.getenv("TEMP") or os.getenv("TMP") or "/tmp"
    dir = dir:gsub("\\", "/")
    if dir:sub(-1) == "/" then
        dir = dir:sub(1, -2)
    end
    local file = io.open(dir .. "/media-hub-mpv-skip", "w")
    if file then
        file:write(which)
        file:close()
    end
end

local function run_button(id)
    if id == "play" then
        mp.commandv("cycle", "pause")
    elseif id == "prev" then
        request_skip("prev")
    elseif id == "next" then
        request_skip("next")
    elseif id == "vol" then
        mp.commandv("cycle", "mute")
    elseif id == "speed" then
        cycle_speed()
    elseif id == "audio" then
        mp.commandv("cycle", "audio")
    elseif id == "sub" then
        mp.commandv("cycle", "sub")
    elseif id == "fs" then
        cycle_fullscreen()
    end
end

local function cancel_pending_pause()
    if pending_pause then
        pending_pause:kill()
        pending_pause = nil
    end
end

local function render()
    local w, h = osd_size()
    if not w or w < 32 or not h or h < 32 then
        return
    end
    local paused = mp.get_property_bool("pause")
    local visible = chrome_visible(paused)
    painted_visible = visible
    local title = ass_escape(mp.get_property("media-title") or "")
    local pos = mp.get_property_number("time-pos") or 0
    local dur = mp.get_property_number("duration") or 0
    local ratio = dur > 0 and math.min(1, math.max(0, pos / dur)) or 0
    local volume = mp.get_property_number("volume") or 100
    local muted = mp.get_property_bool("mute") or volume <= 0
    local speed = mp.get_property_number("speed") or 1
    local sub_on = not track_off(mp.get_property("sid"))
    local full = mp.get_property_bool("fullscreen")
    local mouse = mp.get_property_native("mouse-pos") or {}
    local mx, my = mouse.x or 0, mouse.y or 0
    local geom = layout(w, h)
    local hover_id = mouse.hover and button_at(geom, mx, my) or nil
    local hover_volbar = mouse.hover and geom.volbar.w > 0 and hit(geom.volbar_hit, mx, my)
    local lines = {}

    if visible then
        -- 顶部：薄渐变与标题
        draw_gradient(lines, 0, 0, w, 56, true, 12, 140)
        push(lines, string.format("{\\an7\\bord0\\shad0\\fs17\\b1\\c%s\\pos(%d,18)}%s", COLOR_TEXT, PAD, title))

        -- 底部：平滑渐变背景
        draw_gradient(lines, 0, h - 96, w, 96, false, 16, 200)

        -- 底部进度条
        local seek = geom.seek
        local hover_seek = mouse.hover and hit(seek, mx, my)
        local active = hover_seek or dragging
        local thick = active and 5 or 3
        local bar_cy = seek.y + seek.h / 2
        local bar_top = bar_cy - thick / 2

        push(lines, draw_rect(seek.x, bar_top, seek.w, thick, COLOR_MUTED, "C8"))
        if seek.w * ratio > 0 then
            push(lines, draw_rect(seek.x, bar_top, seek.w * ratio, thick, COLOR_ACCENT, "00"))
        end
        if active then
            push(lines, draw_circle(seek.x + seek.w * ratio, bar_cy, 4.5, COLOR_TEXT, "00"))
        end
        if dur > 0 and active then
            local inspect = mx
            local hover_ratio = math.min(1, math.max(0, (inspect - seek.x) / math.max(1, seek.w)))
            push(lines, string.format(
                "{\\an2\\bord1\\3c&H0A0807&\\shad0\\fs13\\b1\\c%s\\pos(%.1f,%.1f)}%s",
                COLOR_TEXT,
                math.min(seek.x + seek.w, math.max(seek.x, inspect)),
                seek.y - 5,
                clock(hover_ratio * dur)
            ))
        end

        -- 播放时间（当前时间高亮 / 总时长微暗）
        local time_x = geom.nxt.x + geom.nxt.w + 12
        local time_limit = geom.volbar.w > 0 and geom.volbar.x or geom.vol.x
        if time_x + 100 < time_limit then
            local time_str
            if dur > 0 then
                time_str = string.format("{\\c%s}%s{\\c%s} / %s", COLOR_TEXT, clock(pos), COLOR_DIM, clock(dur))
            else
                time_str = string.format("{\\c%s}%s", COLOR_TEXT, clock(pos))
            end
            push(lines, string.format(
                "{\\an4\\bord0\\shad0\\fs14\\pos(%d,%d)}%s",
                time_x,
                geom.play.y + geom.play.h / 2,
                time_str
            ))
        end

        -- 音量滑条（窄窗自适应隐藏）
        if geom.volbar.w > 0 then
            local vol_ratio = math.min(1, math.max(0, volume / 100))
            local vol_active = hover_volbar or dragging_vol
            local vol_thick = vol_active and 5 or 3
            local vol_cy = geom.volbar.y + geom.volbar.h / 2
            local vol_top = vol_cy - vol_thick / 2

            push(lines, draw_rect(geom.volbar.x, vol_top, geom.volbar.w, vol_thick, COLOR_MUTED, "C8"))
            if geom.volbar.w * vol_ratio > 0 then
                push(lines, draw_rect(
                    geom.volbar.x,
                    vol_top,
                    geom.volbar.w * vol_ratio,
                    vol_thick,
                    muted and COLOR_MUTED or COLOR_ACCENT,
                    "00"
                ))
            end
            if vol_active then
                push(lines, draw_circle(
                    geom.volbar.x + geom.volbar.w * vol_ratio,
                    vol_cy,
                    4,
                    COLOR_TEXT,
                    "00"
                ))
            end
        end

        -- 底部功能按钮
        for i = 1, #geom.buttons do
            local box = geom.buttons[i]
            local hot = hover_id == box.id or pressed_id == box.id
            if hot then
                push(lines, draw_circle(box.x + box.w / 2, box.y + box.h / 2, 15, COLOR_TEXT, "E0"))
            end
            local cx, cy = box.x + box.w / 2, box.y + box.h / 2
            local color = hot and COLOR_ACCENT or COLOR_TEXT
            if box.id == "prev" then
                draw_icon_prev(lines, cx, cy, color)
            elseif box.id == "play" then
                if paused then
                    push(lines, draw_icon_play(cx, cy, color))
                else
                    draw_icon_pause(lines, cx, cy, color)
                end
            elseif box.id == "next" then
                draw_icon_next(lines, cx, cy, color)
            elseif box.id == "vol" then
                draw_icon_vol(lines, cx, cy, muted, color)
            elseif box.id == "speed" then
                push(lines, string.format(
                    "{\\an5\\bord0\\shad0\\fs13\\b1\\c%s\\pos(%.1f,%.1f)}%s",
                    color, cx, cy, speed_label(speed)
                ))
            elseif box.id == "audio" then
                push(lines, draw_icon_audio(cx, cy, color))
            elseif box.id == "sub" then
                push(lines, draw_icon_cc(cx, cy, sub_on, sub_on and COLOR_ACCENT or color))
            elseif box.id == "fs" then
                draw_icon_fs(lines, cx, cy, full, color)
            end
        end

        -- 按钮悬浮提示（小巧清晰的底部标签）
        local tip
        local tip_box
        if hover_id == "play" then
            tip = paused and "播放" or "暂停"
            tip_box = geom.play
        elseif hover_id == "prev" then
            tip = "上一集"
            tip_box = geom.prev
        elseif hover_id == "next" then
            tip = "下一集"
            tip_box = geom.nxt
        elseif hover_id == "vol" then
            tip = muted and "取消静音" or string.format("音量 %d%%", math.floor(volume + 0.5))
            tip_box = geom.vol
        elseif hover_id == "speed" then
            tip = "倍速 " .. speed_label(speed)
            tip_box = geom.speed
        elseif hover_id == "audio" then
            tip = track_label("audio")
            tip_box = geom.audio
        elseif hover_id == "sub" then
            tip = track_label("sub")
            tip_box = geom.sub
        elseif hover_id == "fs" then
            tip = full and "退出全屏" or "全屏"
            tip_box = geom.fs
        end
        if tip and tip_box then
            push(lines, string.format(
                "{\\an2\\bord1\\3c&H0A0807&\\shad0\\fs12\\b1\\c%s\\pos(%.1f,%.1f)}%s",
                COLOR_TEXT,
                tip_box.x + tip_box.w / 2,
                tip_box.y - 5,
                ass_escape(tip)
            ))
        end
    end

    if paused then
        local cx, cy = w / 2, h / 2
        push(lines, draw_circle(cx, cy, 26, COLOR_BG, "70"))
        push(lines, draw_icon_play(cx, cy, COLOR_TEXT))
    end

    overlay.res_x = w
    overlay.res_y = h
    overlay.data = table.concat(lines, "\n")
    overlay:update()
end

local function on_mouse(event)
    local w, h = osd_size()
    if not w or not h then
        return
    end
    local geom = layout(w, h)
    local mouse = mp.get_property_native("mouse-pos") or {}
    local mx, my = mouse.x or 0, mouse.y or 0
    local visible = chrome_visible(mp.get_property_bool("pause"))

    if event.event == "down" then
        local id = mouse.hover and button_at(geom, mx, my) or nil
        if id and visible then
            cancel_pending_pause()
            pressed_id = id
            show_chrome()
            render()
            return
        end
        if mouse.hover and visible and geom.volbar.w > 0 and hit(geom.volbar_hit, mx, my) then
            cancel_pending_pause()
            dragging_vol = true
            volume_from_x(geom, mx)
            show_chrome()
            render()
            return
        end
        if mouse.hover and hit(geom.seek, mx, my) and visible then
            cancel_pending_pause()
            dragging = true
            seek_to_ratio((mx - geom.seek.x) / math.max(1, geom.seek.w))
            show_chrome()
            render()
        end
        return
    end
    if event.event ~= "up" then
        return
    end

    if pressed_id then
        local id = mouse.hover and button_at(geom, mx, my) or nil
        if id == pressed_id then
            run_button(id)
        end
        pressed_id = nil
        last_up = 0
        show_chrome()
        render()
        return
    end

    local t = now()
    if t - last_up < 0.28 then
        cancel_pending_pause()
        dragging = false
        dragging_vol = false
        last_up = 0
        cycle_fullscreen()
        show_chrome()
        render()
        return
    end
    last_up = t
    if dragging or dragging_vol then
        dragging = false
        dragging_vol = false
        show_chrome()
        render()
        return
    end
    cancel_pending_pause()
    pending_pause = mp.add_timeout(0.28, function()
        pending_pause = nil
        mp.commandv("cycle", "pause")
        show_chrome()
        render()
    end)
end

mp.add_forced_key_binding("mbtn_left", "hub-click", on_mouse, { complex = true })
mp.add_forced_key_binding("f", "hub-fullscreen", cycle_fullscreen)
mp.add_forced_key_binding("<", "hub-prev", function() request_skip("prev") end)
mp.add_forced_key_binding(">", "hub-next", function() request_skip("next") end)

mp.observe_property("fullscreen", "bool", function(_, fs)
    if fs ~= nil then
        mp.set_property_bool("border", not fs)
    end
    show_chrome()
    render()
end)

mp.observe_property("mouse-pos", "native", function(_, value)
    local w, h = osd_size()
    if not w or not h then
        return
    end
    local geom = layout(w, h)
    if dragging and value then
        seek_to_ratio(((value.x or 0) - geom.seek.x) / math.max(1, geom.seek.w))
        show_chrome()
        render()
        return
    end
    if dragging_vol and value then
        volume_from_x(geom, value.x or 0)
        show_chrome()
        render()
        return
    end
    if value and value.hover then
        local was = chrome_visible(mp.get_property_bool("pause"))
        show_chrome()
        local x, y = value.x or 0, value.y or 0
        if not was or button_at(geom, x, y) or hit(geom.seek, x, y) or (geom.volbar.w > 0 and hit(geom.volbar_hit, x, y)) then
            render()
        end
    end
end)

mp.observe_property("pause", "bool", render)
mp.observe_property("mute", "bool", render)
mp.observe_property("volume", "number", render)
mp.observe_property("speed", "number", render)
mp.observe_property("aid", "native", render)
mp.observe_property("sid", "native", render)
mp.observe_property("time-pos", "number", render)
mp.observe_property("duration", "number", render)
mp.observe_property("osd-dimensions", "native", render)
mp.observe_property("media-title", "string", render)
mp.register_event("file-loaded", function()
    show_chrome()
    render()
end)
mp.add_periodic_timer(0.15, function()
    if dragging or dragging_vol or mp.get_property_bool("pause") then
        return
    end
    if painted_visible and now() >= chrome_until then
        render()
    end
end)
show_chrome()
render()
