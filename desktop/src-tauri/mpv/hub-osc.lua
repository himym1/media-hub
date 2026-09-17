-- Hub 控制条。每条 ASS 事件必须单独换行，否则后续 \pos 会全部叠到左上角。
local overlay = mp.create_osd_overlay("ass-events")
local chrome_until = 0
local dragging = false
local last_up = 0
local pending_pause = nil
local painted_visible = true

local COLOR_BG = "&H0A0807&"
local COLOR_TEXT = "&HEBF1F2&"
local COLOR_MUTED = "&HBFC6C4&"
local COLOR_ACCENT = "&H99D334&"
local PAD = 40
local BAR_H = 4

local function now()
    return mp.get_time()
end

local function show_chrome()
    chrome_until = now() + 2.0
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

local function bar_layout(w, h)
    local bar_y = h - 36
    local x1 = PAD + 52
    local x2 = w - PAD - 52
    if x2 <= x1 + 24 then
        x1 = PAD
        x2 = w - PAD
    end
    return x1, bar_y, x2
end

local function in_seekbar(x, y, x1, bar_y, x2)
    return x >= x1 - 8 and x <= x2 + 8 and y >= bar_y - 18 and y <= bar_y + 18
end

local function cancel_pending_pause()
    if pending_pause then
        pending_pause:kill()
        pending_pause = nil
    end
end

local function seek_to_ratio(ratio)
    mp.commandv("seek", math.min(1, math.max(0, ratio)) * 100, "absolute-percent")
end

local function cycle_fullscreen()
    mp.commandv("cycle", "fullscreen")
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
    local lines = {}

    if visible then
        draw_gradient(lines, 0, 0, w, 72, true, 8, 180)
        push(lines, string.format("{\\an7\\bord0\\shad0\\fs22\\c%s\\pos(%d,22)}%s", COLOR_TEXT, PAD, title))

        draw_gradient(lines, 0, h - 88, w, 88, false, 8, 210)

        local x1, bar_y, x2 = bar_layout(w, h)
        local span = math.max(1, x2 - x1)
        local mouse = mp.get_property_native("mouse-pos") or {}
        local hover = mouse.hover and in_seekbar(mouse.x or 0, mouse.y or 0, x1, bar_y, x2)
        local active = hover or dragging
        local thick = active and 6 or BAR_H
        local bar_top = bar_y - thick / 2

        push(lines, string.format("{\\an1\\bord0\\shad0\\fs16\\c%s\\pos(%d,%d)}%s", COLOR_MUTED, PAD, h - 14, clock(pos)))
        if dur > 0 then
            push(lines, string.format("{\\an3\\bord0\\shad0\\fs16\\c%s\\pos(%d,%d)}%s", COLOR_MUTED, w - PAD, h - 14, clock(dur)))
        end
        push(lines, draw_rect(x1, bar_top, span, thick, COLOR_MUTED, "B0"))
        if span * ratio > 0 then
            push(lines, draw_rect(x1, bar_top, span * ratio, thick, COLOR_ACCENT, "00"))
        end
        if active then
            push(lines, draw_circle(x1 + span * ratio, bar_y, 5, COLOR_TEXT, "00"))
        end
        if dur > 0 and (hover or dragging) then
            local inspect = dragging and (mouse.x or (x1 + span * ratio)) or (mouse.x or x1)
            local hover_ratio = math.min(1, math.max(0, (inspect - x1) / span))
            push(lines, string.format(
                "{\\an2\\bord0\\shad0\\fs14\\c%s\\pos(%.1f,%.1f)}%s",
                COLOR_TEXT,
                math.min(x2, math.max(x1, inspect)),
                bar_y - 16,
                clock(hover_ratio * dur)
            ))
        end
    end

    if paused then
        local cx, cy = w / 2, h / 2
        push(lines, draw_circle(cx, cy, 36, COLOR_BG, "50"))
        push(lines, string.format(
            "{\\an7\\bord0\\shad0\\p1\\c%s\\pos(%.1f,%.1f)}m 0 0 l 22 13 l 0 26{\\p0}",
            COLOR_TEXT, cx - 7, cy - 13
        ))
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
    local x1, bar_y, x2 = bar_layout(w, h)
    if event.event == "down" then
        local mouse = mp.get_property_native("mouse-pos") or {}
        if mouse.hover and in_seekbar(mouse.x or 0, mouse.y or 0, x1, bar_y, x2) and chrome_visible(mp.get_property_bool("pause")) then
            cancel_pending_pause()
            dragging = true
            seek_to_ratio(((mouse.x or 0) - x1) / math.max(1, x2 - x1))
            show_chrome()
            render()
        end
        return
    end
    if event.event ~= "up" then
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
        -- 全屏去掉系统边框，才是真正铺满；窗口模式保留红绿灯。
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
    local x1, bar_y, x2 = bar_layout(w, h)
    if dragging and value then
        seek_to_ratio(((value.x or 0) - x1) / math.max(1, x2 - x1))
        show_chrome()
        render()
        return
    end
    if value and value.hover then
        local was = chrome_visible(mp.get_property_bool("pause"))
        show_chrome()
        if not was or in_seekbar(value.x or 0, value.y or 0, x1, bar_y, x2) then
            render()
        end
    end
end)

mp.observe_property("pause", "bool", render)
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
