# Android 应用

Media Hub 的原生 Android 客户端。

- Kotlin 2.4.10
- Jetpack Compose
- Miuix 0.9.3，随功能增长隔离在项目自有 UI 封装之后
- Lucide Compose 图标
- Navigation 3 稳定版

客户端通过共用的已认证 Media Hub API 完成发现、订阅、转存、媒体库访问、服务状态、115 操作和迁移恢复。会话令牌在私有持久化之前先用 Android Keystore 加密；正式构建会拒绝明文 API URL。

签名正式 APK 默认指向 `https://media.himym.us.ci`，全新安装会直接进入登录流程，无需填写服务器地址。用户保存的服务器 URL 优先；服务页仍保留显式更换服务器流程，用于迁移或恢复部署。
