-- Hub 电影风控制条：深底、翠绿进度、暂停居中键。不走 mpv 自带灰条 OSC。
local overlay = mp.create_osd_overlay("ass-events")
local chrome_until = 0
local dragging = false
local last_up = 0
local pending_pause = nil
local painted_visible = true
local PAD = 56
local BAR_H = 6

local function now()
    return mp.get_time()
end

local function show_chrome()
    chrome_until = now() + 2.2
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

local function bar_rect(w, h)
    return PAD, h - 44, w - PAD, h - 44 + BAR_H
end

local function in_seekbar(x, y, w, h)
    local x1, y1, x2 = bar_rect(w, h)
    return x >= x1 - 8 and x <= x2 + 8 and y >= y1 - 18 and y <= h - 10
end

local function seek_at(x, w)
    local span = w - PAD * 2
    if span <= 1 then
        return
    end
    local ratio = math.min(1, math.max(0, (x - PAD) / span))
    mp.commandv("seek", ratio * 100, "absolute-percent")
end

local function box(x, y, w, h, color, alpha)
    if w <= 0 or h <= 0 then
        return ""
    end
    return string.format(
        "{\\an7\\bord0\\shad0\\p1\\c%s\\alpha&H%s&\\pos(%.1f,%.1f)}m 0 0 l %.1f 0 l %.1f %.1f l 0 %.1f{\\p0}",
        color, alpha, x, y, w, w, h, h
    )
end

local function cancel_pending_pause()
    if pending_pause then
        pending_pause:kill()
        pending_pause = nil
    end
end

local function render()
    local w, h = mp.get_osd_size()
    if not w or w < 8 or not h or h < 8 then
        return
    end
    local paused = mp.get_property_bool("pause")
    local visible = chrome_visible(paused)
    painted_visible = visible
    local title = ass_escape(mp.get_property("media-title") or "")
    local pos = mp.get_property_number("time-pos") or 0
    local dur = mp.get_property_number("duration") or 0
    local ratio = 0
    if dur > 0 then
        ratio = math.min(1, math.max(0, pos / dur))
    end
    local lines = {}
    if visible then
        lines[#lines + 1] = box(0, 0, w, 88, "&H070605&", "96")
        lines[#lines + 1] = string.format(
            "{\\an7\\bord0\\shad0\\fs28\\b1\\c&HEBF1F2&\\pos(28,22)}%s",
            title
        )
        lines[#lines + 1] = box(0, h - 96, w, 96, "&H070605&", "4A")
        local x1, y1, x2 = bar_rect(w, h)
        local span = x2 - x1
        lines[#lines + 1] = box(x1, y1, span, BAR_H, "&H3D3A36&", "00")
        lines[#lines + 1] = box(x1, y1, span * ratio, BAR_H, "&H99D334&", "00")
        local knob = 11
        lines[#lines + 1] = box(x1 + span * ratio - knob / 2, y1 - 2.5, knob, knob, "&HEBF1F2&", "00")
        lines[#lines + 1] = string.format(
            "{\\an1\\bord0\\shad0\\fs20\\c&HEBF1F2&\\pos(%d,%d)}%s",
            PAD, h - 18, clock(pos)
        )
        if dur > 0 then
            lines[#lines + 1] = string.format(
                "{\\an3\\bord0\\shad0\\fs20\\c&HEBF1F2&\\pos(%d,%d)}%s",
                w - PAD, h - 18, clock(dur)
            )
        end
    end
    if paused then
        local cx, cy = w / 2, h / 2
        lines[#lines + 1] = box(cx - 44, cy - 44, 88, 88, "&H070605&", "40")
        lines[#lines + 1] = string.format(
            "{\\an7\\bord0\\shad0\\p1\\c&HEBF1F2&\\pos(%.1f,%.1f)}m 0 0 l 32 20 l 0 40{\\p0}",
            cx - 10, cy - 20
        )
    end
    overlay.res_x = w
    overlay.res_y = h
    overlay.data = table.concat(lines)
    overlay:update()
end

local function on_mouse(event)
    if event.event == "down" then
        local mouse = mp.get_property_native("mouse-pos") or {}
        local w, h = mp.get_osd_size()
        if mouse.hover and in_seekbar(mouse.x or 0, mouse.y or 0, w, h) and chrome_visible(mp.get_property_bool("pause")) then
            cancel_pending_pause()
            dragging = true
            seek_at(mouse.x or 0, w)
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
        mp.commandv("cycle", "fullscreen")
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
mp.observe_property("mouse-pos", "native", function(_, value)
    if dragging and value then
        local w = mp.get_osd_size()
        seek_at(value.x or 0, w)
        show_chrome()
        render()
        return
    end
    if value and value.hover then
        local was = chrome_visible(mp.get_property_bool("pause"))
        show_chrome()
        if not was then
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
