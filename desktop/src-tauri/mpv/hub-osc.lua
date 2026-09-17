-- Hub 电影风控制条：上下渐变、单行进度、暂停居中圆钮。

local overlay = mp.create_osd_overlay("ass-events")
local chrome_until = 0
local dragging = false
local last_up = 0
local pending_pause = nil
local painted_visible = true

-- 视觉常量（遵循 Hub 电影风色彩与层级规范）
local COLOR_BG = "&H0A0807&"      -- #07080A 深黑底色
local COLOR_TEXT = "&HEBF1F2&"    -- #f2f1eb 主文字高光
local COLOR_MUTED = "&HBFC6C4&"   -- #c4c6bf 次要文字与轨道底
local COLOR_ACCENT = "&H99D334&"  -- #34d399 翡翠绿强调
local COLOR_WHITE = "&HFFFFFF&"   -- 纯白外轮廓微光

local PAD_X = 40                  -- 两侧内边距
local TOP_GRAD_H = 104            -- 顶部遮罩高度
local BOT_GRAD_H = 128            -- 底部遮罩高度
local BAR_H = 3                   -- 细进度条基准高度
local BAR_ACTIVE_H = 5            -- 悬停/拖拽时进度条饱满高度
local HIT_Y_PAD = 16              -- 进度条垂直点击容差（保证触控区至少 35px+）

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
    return string.format("%02d:%02d", m, s)
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
    -- 标准四象限三次贝塞尔精密圆形
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

-- 绘制精致黑曜石圆盘（带 1px 细微光外边框与半透明深黑基底）
local function draw_disc(cx, cy, r, bg_color, bg_alpha, border_color, border_alpha)
    local k = r * 0.55228
    return string.format(
        "{\\an7\\bord1\\3c%s\\3a&H%s&\\1c%s\\1a&H%s&\\shad0\\p1\\pos(%.1f,%.1f)}"
            .. "m %.1f 0 "
            .. "b %.1f %.1f %.1f %.1f 0 %.1f "
            .. "b %.1f %.1f %.1f %.1f %.1f 0 "
            .. "b %.1f %.1f %.1f %.1f 0 %.1f "
            .. "b %.1f %.1f %.1f %.1f %.1f 0{\\p0}",
        border_color, border_alpha, bg_color, bg_alpha, cx, cy,
        r,
        r, k, k, r, r,
        -k, r, -r, r, -r,
        -r, -k, -k, -r, -r,
        k, -r, r, -r, r
    )
end

-- 绘制圆角胶囊药丸（带 1px 外轮廓微光，用于 Hover 时间提示背景）
local function draw_pill(cx, cy, w, h, r, bg_color, bg_alpha, border_color, border_alpha)
    local x = cx - w / 2
    local y = cy - h / 2
    r = math.min(r, h / 2, w / 2)
    local k = r * 0.55228
    return string.format(
        "{\\an7\\bord1\\3c%s\\3a&H%s&\\1c%s\\1a&H%s&\\shad0\\p1\\pos(%.1f,%.1f)}"
            .. "m %.1f 0 "
            .. "l %.1f 0 "
            .. "b %.1f 0 %.1f %.1f %.1f %.1f "
            .. "l %.1f %.1f "
            .. "b %.1f %.1f %.1f %.1f %.1f %.1f "
            .. "l %.1f %.1f "
            .. "b %.1f %.1f 0 %.1f 0 %.1f "
            .. "l 0 %.1f "
            .. "b 0 %.1f %.1f 0 %.1f 0{\\p0}",
        border_color, border_alpha, bg_color, bg_alpha, x, y,
        r,
        w - r,
        w - r + k, w, r - k, w, r,
        w, h - r,
        w, h - r + k, w - r + k, h, w - r, h,
        r, h,
        r - k, h, h - r + k, h - r,
        r,
        r - k, r - k, r
    )
end

-- 电影级非线性渐变遮罩：避免线性分层阶梯感，提供温润漫反射暗角
local function draw_film_gradient(x, y, w, h, is_top, steps, base_alpha)
    local lines = {}
    local step_h = h / steps
    for i = 0, steps - 1 do
        local t = i / (steps - 1)
        -- 非线性三次衰减：让靠近边缘处更深邃，远离处轻柔消隐
        local decay = (1.0 - t) * (1.0 - t) * (1.0 - t * 0.4)
        local a = math.floor(base_alpha * decay)
        if a > 0 then
            local hex_alpha = string.format("%02X", math.max(0, math.min(255, 255 - a)))
            local cur_y = is_top and (y + i * step_h) or (y + h - (i + 1) * step_h)
            lines[#lines + 1] = draw_rect(x, cur_y, w, step_h + 0.5, COLOR_BG, hex_alpha)
        end
    end
    return table.concat(lines)
end

-- 计算底部排版几何（时间 · 细进度 · 时长 一体化流线）
local function get_bottom_geometry(w, h, pos, dur)
    local bar_y = h - 30
    local pos_str = clock(pos)
    local dur_str = clock(dur)
    local pos_w = (pos >= 3600) and 64 or 46
    local dur_w = (dur >= 3600) and 64 or 46
    local gap = 16
    local x1 = PAD_X + pos_w + gap
    local x2 = (dur > 0) and (w - PAD_X - dur_w - gap) or (w - PAD_X)
    if x2 <= x1 + 30 then
        x1 = PAD_X
        x2 = w - PAD_X
    end
    return x1, bar_y, x2, pos_str, dur_str
end

local function in_seekbar_hitbox(x, y, x1, bar_y, x2)
    return x >= x1 - 10 and x <= x2 + 10 and y >= bar_y - HIT_Y_PAD and y <= bar_y + HIT_Y_PAD
end

local function cancel_pending_pause()
    if pending_pause then
        pending_pause:kill()
        pending_pause = nil
    end
end

local function seek_to_ratio(ratio)
    local clamped = math.min(1, math.max(0, ratio))
    mp.commandv("seek", clamped * 100, "absolute-percent", "exact")
end

local function render()
    local w, h = mp.get_osd_size()
    if not w or w < 32 or not h or h < 32 then
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
        -- 1. 顶部电影级薄渐变遮罩与片名
        lines[#lines + 1] = draw_film_gradient(0, 0, w, TOP_GRAD_H, true, 20, 200)
        lines[#lines + 1] = string.format(
            "{\\an7\\bord0\\shad1\\4c&H000000&\\4a&H80&\\fs22\\c%s\\pos(%d,26)}%s",
            COLOR_TEXT, PAD_X, title
        )

        -- 2. 底部电影级渐变遮罩
        lines[#lines + 1] = draw_film_gradient(0, h - BOT_GRAD_H, w, BOT_GRAD_H, false, 24, 225)

        -- 3. 单行流线排版：时间 · 细进度 · 时长
        local x1, bar_y, x2, pos_str, dur_str = get_bottom_geometry(w, h, pos, dur)
        local span = math.max(1, x2 - x1)

        local mouse = mp.get_property_native("mouse-pos") or {}
        local is_hover = mouse.hover and in_seekbar_hitbox(mouse.x or 0, mouse.y or 0, x1, bar_y, x2)
        local is_active = is_hover or dragging
        local current_bar_h = is_active and BAR_ACTIVE_H or BAR_H
        local bar_top_y = bar_y - current_bar_h / 2

        -- 左侧当前时间（垂直居中对齐）
        lines[#lines + 1] = string.format(
            "{\\an4\\bord0\\shad0\\fs14\\c%s\\pos(%d,%d)}%s",
            COLOR_MUTED, PAD_X, bar_y, pos_str
        )

        -- 右侧总时长（垂直居中右对齐）
        if dur > 0 then
            lines[#lines + 1] = string.format(
                "{\\an6\\bord0\\shad0\\fs14\\c%s\\pos(%d,%d)}%s",
                COLOR_MUTED, w - PAD_X, bar_y, dur_str
            )
        end

        -- 中间细进度槽（背景底色微透柔和）
        lines[#lines + 1] = draw_rect(x1, bar_top_y, span, current_bar_h, COLOR_MUTED, "B0")

        -- 已播放翡翠绿进度条
        local played_w = span * ratio
        if played_w > 0 then
            lines[#lines + 1] = draw_rect(x1, bar_top_y, played_w, current_bar_h, COLOR_ACCENT, "00")
        end

        -- 进度条滑块 Knob（悬停或拖拽时优雅浮现）
        if is_active then
            local knob_x = x1 + played_w
            -- 外圈翡翠柔光环
            lines[#lines + 1] = draw_circle(knob_x, bar_y, 6.5, COLOR_ACCENT, "60")
            -- 内圈纯白实心圆点
            lines[#lines + 1] = draw_circle(knob_x, bar_y, 4.0, COLOR_TEXT, "00")
        end

        -- 4. 悬停时间胶囊气泡（Hover / Dragging 浮动提示）
        local inspect_x = dragging and (mouse.x or (x1 + played_w)) or (is_hover and mouse.x)
        if inspect_x and dur > 0 then
            local hover_ratio = math.min(1, math.max(0, (inspect_x - x1) / span))
            local hover_seconds = hover_ratio * dur
            local hover_str = clock(hover_seconds)
            local pill_w = (dur >= 3600) and 68 or 52
            local pill_h = 22
            local bubble_cx = math.min(x2 - pill_w / 2, math.max(x1 + pill_w / 2, inspect_x))
            local bubble_cy = bar_y - 24

            -- 黑曜石半透明微透胶囊（带 1px 细微光外框）
            lines[#lines + 1] = draw_pill(bubble_cx, bubble_cy, pill_w, pill_h, 6, COLOR_BG, "38", COLOR_WHITE, "DC")
            -- 气泡内时间文本
            lines[#lines + 1] = string.format(
                "{\\an5\\bord0\\shad0\\fs12\\c%s\\pos(%.1f,%.1f)}%s",
                COLOR_TEXT, bubble_cx, bubble_cy, hover_str
            )
        end
    end

    -- 5. 暂停时：中央半透明黑曜石底盘 + 矢量播放三角形
    if paused then
        local cx, cy = w / 2, h / 2
        -- 核心黑曜石底盘（带 1px 纯白微亮外环）
        lines[#lines + 1] = draw_disc(cx, cy, 37, COLOR_BG, "48", COLOR_WHITE, "D4")
        -- 视差修正居中播放三角（底边 22，高 20，向右微调 1.5px 保持光学重心居中）
        lines[#lines + 1] = string.format(
            "{\\an7\\bord0\\shad0\\p1\\c%s\\alpha&H00&\\pos(%.1f,%.1f)}"
                .. "m 0 0 "
                .. "l 19 11 "
                .. "l 0 22 "
                .. "l 0 0{\\p0}",
            COLOR_TEXT, cx - 6.5, cy - 11
        )
    end

    overlay.res_x = w
    overlay.res_y = h
    overlay.data = table.concat(lines)
    overlay:update()
end

local function on_mouse(event)
    local w, h = mp.get_osd_size()
    if not w or not h then return end

    local pos = mp.get_property_number("time-pos") or 0
    local dur = mp.get_property_number("duration") or 0
    local x1, bar_y, x2 = get_bottom_geometry(w, h, pos, dur)

    if event.event == "down" then
        local mouse = mp.get_property_native("mouse-pos") or {}
        local mx = mouse.x or 0
        local my = mouse.y or 0
        if mouse.hover and in_seekbar_hitbox(mx, my, x1, bar_y, x2) and chrome_visible(mp.get_property_bool("pause")) then
            cancel_pending_pause()
            dragging = true
            local span = math.max(1, x2 - x1)
            seek_to_ratio((mx - x1) / span)
            show_chrome()
            render()
        end
        return
    end

    if event.event ~= "up" then
        return
    end

    local t = now()
    -- 双击全屏检测（0.28s 窗口）
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

    -- 拖拽释放
    if dragging then
        dragging = false
        show_chrome()
        render()
        return
    end

    -- 单击延迟触发暂停，避免全屏手势冲突
    cancel_pending_pause()
    pending_pause = mp.add_timeout(0.28, function()
        pending_pause = nil
        mp.commandv("cycle", "pause")
        show_chrome()
        render()
    end)
end

-- 注册左键由 Hub 控制条独占管理
mp.add_forced_key_binding("mbtn_left", "hub-click", on_mouse, { complex = true })

mp.observe_property("mouse-pos", "native", function(_, value)
    local w, h = mp.get_osd_size()
    if not w or not h then return end

    local pos = mp.get_property_number("time-pos") or 0
    local dur = mp.get_property_number("duration") or 0
    local x1, bar_y, x2 = get_bottom_geometry(w, h, pos, dur)
    local span = math.max(1, x2 - x1)

    if dragging and value then
        seek_to_ratio(((value.x or 0) - x1) / span)
        show_chrome()
        render()
        return
    end

    if value and value.hover then
        local was = chrome_visible(mp.get_property_bool("pause"))
        show_chrome()
        if not was then
            render()
        else
            if in_seekbar_hitbox(value.x or 0, value.y or 0, x1, bar_y, x2) then
                render()
            end
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
