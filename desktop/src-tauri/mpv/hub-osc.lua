-- Hub 控制条。每条 ASS 事件必须单独换行，否则后续 \pos 会全部叠到左上角。
local overlay = mp.create_osd_overlay("ass-events")
local chrome_until = 0
local dragging = false
local last_up = 0
local pending_pause = nil
local painted_visible = true
local pressed_id = nil

local COLOR_BG = "&H0A0807&"
local COLOR_TEXT = "&HEBF1F2&"
local COLOR_MUTED = "&HBFC6C4&"
local COLOR_ACCENT = "&H99D334&"
local PAD = 28
local BTN = 36

local function now()
    return mp.get_time()
end

local function show_chrome()
    chrome_until = now() + 2.4
end

local function chrome_visible(paused)
    return paused or now() < chrome_until or dragging
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
            push(lines, draw_rect(x, cur_y, w, step_h + 0.5, COLOR_BG, string.format("%02X", 255 - a)))
        end
    end
end

local function hit(box, x, y)
    return x >= box.x and x <= box.x + box.w and y >= box.y and y <= box.y + box.h
end

local function layout(w, h)
    local row_y = h - 44
    local play = { id = "play", x = PAD, y = row_y, w = BTN, h = BTN }
    local fs = { id = "fs", x = w - PAD - BTN, y = row_y, w = BTN, h = BTN }
    local sub = { id = "sub", x = fs.x - 8 - BTN, y = row_y, w = BTN, h = BTN }
    local vol = { id = "vol", x = sub.x - 8 - BTN, y = row_y, w = BTN, h = BTN }
    local seek = {
        id = "seek",
        x = PAD,
        y = h - 78,
        w = w - PAD * 2,
        h = 22,
    }
    return {
        play = play,
        vol = vol,
        sub = sub,
        fs = fs,
        seek = seek,
        buttons = { play, vol, sub, fs },
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
        "{\\an7\\bord0\\shad0\\p1\\c%s\\pos(%.1f,%.1f)}m 0 0 l 16 9 l 0 18{\\p0}",
        color, cx - 5, cy - 9
    )
end

local function draw_icon_pause(lines, cx, cy, color)
    push(lines, draw_rect(cx - 7, cy - 8, 5, 16, color, "00"))
    push(lines, draw_rect(cx + 2, cy - 8, 5, 16, color, "00"))
end

local function draw_icon_vol(lines, cx, cy, muted, color)
    push(lines, string.format(
        "{\\an7\\bord0\\shad0\\p1\\c%s\\pos(%.1f,%.1f)}m 0 6 l 6 6 l 12 1 l 12 17 l 6 12 l 0 12{\\p0}",
        color, cx - 11, cy - 9
    ))
    if muted then
        push(lines, string.format(
            "{\\an7\\bord0\\shad0\\p1\\c%s\\pos(%.1f,%.1f)}m 0 0 l 10 10 m 10 0 l 0 10{\\p0}",
            color, cx + 2, cy - 5
        ))
        return
    end
    push(lines, string.format(
        "{\\an7\\bord0\\shad0\\p1\\c%s\\pos(%.1f,%.1f)}m 0 3 l 5 0 m 0 8 l 6 8 m 0 13 l 5 16{\\p0}",
        color, cx + 3, cy - 8
    ))
end

local function draw_icon_cc(cx, cy, on, color)
    local label = on and "CC" or "Cc"
    return string.format("{\\an5\\bord0\\shad0\\fs15\\b1\\c%s\\pos(%.1f,%.1f)}%s", color, cx, cy, label)
end

local function draw_icon_fs(cx, cy, full, color)
    if full then
        return string.format(
            "{\\an7\\bord0\\shad0\\p1\\c%s\\pos(%.1f,%.1f)}"
                .. "m 6 0 l 6 6 l 0 6 m 14 0 l 14 6 l 20 6 m 0 14 l 6 14 l 6 20 m 14 20 l 14 14 l 20 14{\\p0}",
            color, cx - 10, cy - 10
        )
    end
    return string.format(
        "{\\an7\\bord0\\shad0\\p1\\c%s\\pos(%.1f,%.1f)}"
            .. "m 0 6 l 0 0 l 6 0 m 14 0 l 20 0 l 20 6 m 0 14 l 0 20 l 6 20 m 14 20 l 20 20 l 20 14{\\p0}",
        color, cx - 10, cy - 10
    )
end

local function seek_to_ratio(ratio)
    mp.commandv("seek", math.min(1, math.max(0, ratio)) * 100, "absolute-percent")
end

local function cycle_fullscreen()
    mp.commandv("cycle", "fullscreen")
end

local function run_button(id)
    if id == "play" then
        mp.commandv("cycle", "pause")
    elseif id == "vol" then
        mp.commandv("cycle", "mute")
    elseif id == "sub" then
        mp.commandv("cycle", "sub-visibility")
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
    local muted = mp.get_property_bool("mute") or (mp.get_property_number("volume") or 100) <= 0
    local sub_on = mp.get_property_bool("sub-visibility")
    local full = mp.get_property_bool("fullscreen")
    local mouse = mp.get_property_native("mouse-pos") or {}
    local mx, my = mouse.x or 0, mouse.y or 0
    local geom = layout(w, h)
    local hover_id = mouse.hover and button_at(geom, mx, my) or nil
    local lines = {}

    if visible then
        draw_gradient(lines, 0, 0, w, 72, true, 8, 180)
        push(lines, string.format("{\\an7\\bord0\\shad0\\fs22\\c%s\\pos(%d,22)}%s", COLOR_TEXT, PAD, title))
        draw_gradient(lines, 0, h - 108, w, 108, false, 8, 210)

        local seek = geom.seek
        local hover_seek = mouse.hover and hit(seek, mx, my)
        local active = hover_seek or dragging
        local thick = active and 6 or 4
        local bar_top = seek.y + (seek.h - thick) / 2
        push(lines, draw_rect(seek.x, bar_top, seek.w, thick, COLOR_MUTED, "B0"))
        if seek.w * ratio > 0 then
            push(lines, draw_rect(seek.x, bar_top, seek.w * ratio, thick, COLOR_ACCENT, "00"))
        end
        if active then
            push(lines, draw_circle(seek.x + seek.w * ratio, seek.y + seek.h / 2, 5, COLOR_TEXT, "00"))
        end
        if dur > 0 and (hover_seek or dragging) then
            local inspect = dragging and mx or mx
            local hover_ratio = math.min(1, math.max(0, (inspect - seek.x) / math.max(1, seek.w)))
            push(lines, string.format(
                "{\\an2\\bord0\\shad0\\fs14\\c%s\\pos(%.1f,%.1f)}%s",
                COLOR_TEXT,
                math.min(seek.x + seek.w, math.max(seek.x, inspect)),
                seek.y - 4,
                clock(hover_ratio * dur)
            ))
        end

        local time_x = geom.play.x + geom.play.w + 10
        push(lines, string.format(
            "{\\an4\\bord0\\shad0\\fs16\\c%s\\pos(%d,%d)}%s%s%s",
            COLOR_MUTED,
            time_x,
            geom.play.y + geom.play.h / 2,
            clock(pos),
            dur > 0 and " / " or "",
            dur > 0 and clock(dur) or ""
        ))

        for i = 1, #geom.buttons do
            local box = geom.buttons[i]
            local hot = hover_id == box.id or pressed_id == box.id
            if hot then
                push(lines, draw_circle(box.x + box.w / 2, box.y + box.h / 2, 16, COLOR_TEXT, "E6"))
            end
            local cx, cy = box.x + box.w / 2, box.y + box.h / 2
            local color = hot and COLOR_ACCENT or COLOR_TEXT
            if box.id == "play" then
                if paused then
                    push(lines, draw_icon_play(cx, cy, color))
                else
                    draw_icon_pause(lines, cx, cy, color)
                end
            elseif box.id == "vol" then
                draw_icon_vol(lines, cx, cy, muted, color)
            elseif box.id == "sub" then
                push(lines, draw_icon_cc(cx, cy, sub_on, sub_on and COLOR_ACCENT or color))
            elseif box.id == "fs" then
                push(lines, draw_icon_fs(cx, cy, full, color))
            end
        end
    end

    if paused then
        local cx, cy = w / 2, h / 2
        push(lines, draw_circle(cx, cy, 36, COLOR_BG, "50"))
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

    if event.event == "down" then
        local id = mouse.hover and button_at(geom, mx, my) or nil
        if id and chrome_visible(mp.get_property_bool("pause")) then
            cancel_pending_pause()
            pressed_id = id
            show_chrome()
            render()
            return
        end
        if mouse.hover and hit(geom.seek, mx, my) and chrome_visible(mp.get_property_bool("pause")) then
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
        last_up = 0
        cycle_fullscreen()
        show_chrome()
        render()
        return
    end
    last_up = t
    if dragging then
        dragging = false
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
    if value and value.hover then
        local was = chrome_visible(mp.get_property_bool("pause"))
        show_chrome()
        if not was or button_at(geom, value.x or 0, value.y or 0) or hit(geom.seek, value.x or 0, value.y or 0) then
            render()
        end
    end
end)

mp.observe_property("pause", "bool", render)
mp.observe_property("mute", "bool", render)
mp.observe_property("volume", "number", render)
mp.observe_property("sub-visibility", "bool", render)
mp.observe_property("time-pos", "number", render)
mp.observe_property("duration", "number", render)
mp.observe_property("osd-dimensions", "native", render)
mp.observe_property("media-title", "string", render)
mp.register_event("file-loaded", function()
    show_chrome()
    render()
end)
mp.add_periodic_timer(0.15, function()
    if dragging or mp.get_property_bool("pause") then
        return
    end
    if painted_visible and now() >= chrome_until then
        render()
    end
end)
show_chrome()
render()
