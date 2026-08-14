import java.util.Base64
import java.util.Properties
plugins {
    alias(libs.plugins.android.application)
    alias(libs.plugins.kotlin.compose)
}

val releaseProperties = Properties().apply {
    val encoded = providers.environmentVariable("MEDIA_HUB_ANDROID_KEYSTORE_BASE64").orNull
    if (!encoded.isNullOrBlank()) {
        val output = rootProject.layout.projectDirectory.file(".gradle/signing/media-hub-release.jks").asFile
        output.parentFile.mkdirs()
        output.writeBytes(Base64.getDecoder().decode(encoded))
        setProperty("storeFile", output.absolutePath)
        setProperty("storePassword", providers.environmentVariable("MEDIA_HUB_ANDROID_KEYSTORE_PASSWORD").get())
        setProperty("keyAlias", providers.environmentVariable("MEDIA_HUB_ANDROID_KEY_ALIAS").get())
        setProperty("keyPassword", providers.environmentVariable("MEDIA_HUB_ANDROID_KEY_PASSWORD").get())
    }
}

val releaseVersionCode = providers.gradleProperty("MEDIA_HUB_VERSION_CODE").orElse("13006").get().toInt()
val releaseVersionName = providers.gradleProperty("MEDIA_HUB_VERSION_NAME").orElse("0.13.6").get()

val mediaHubApiBaseUrl = providers.gradleProperty("MEDIA_HUB_API_BASE_URL")
    .orElse("https://media.himym.us.ci")
    .get()
    .replace("\\", "\\\\")
    .replace("\"", "\\\"")

android {
    namespace = "com.mediahub.android"
    compileSdk = 37

    defaultConfig {
        applicationId = "com.mediahub.android"
        minSdk = 26
        targetSdk = 37
        versionCode = releaseVersionCode
        versionName = releaseVersionName
        buildConfigField("String", "API_BASE_URL", "\"$mediaHubApiBaseUrl\"")
    }

    signingConfigs {
        if (releaseProperties.isNotEmpty()) {
            create("release") {
                storeFile = file(releaseProperties.getProperty("storeFile"))
                storePassword = releaseProperties.getProperty("storePassword")
                keyAlias = releaseProperties.getProperty("keyAlias")
                keyPassword = releaseProperties.getProperty("keyPassword")
            }
        }
    }

    buildTypes {
        getByName("debug") {
            buildConfigField("String", "API_BASE_URL", "\"http://10.0.2.2:8080\"")
        }
        getByName("release") {
            signingConfig = signingConfigs.findByName("release")
            isMinifyEnabled = false
        }
    }

    buildFeatures {
        compose = true
        buildConfig = true
    }

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }

    packaging {
        resources.excludes += "/META-INF/{AL2.0,LGPL2.1}"
    }
}

dependencies {
    implementation(libs.androidx.core)
    implementation(libs.androidx.activity.compose)
    implementation(libs.androidx.lifecycle.viewmodel.compose)
    implementation(libs.androidx.compose.foundation)
    implementation(libs.kotlinx.coroutines.android)
    implementation(libs.miuix.ui)
    implementation(libs.miuix.squircle)
    implementation(libs.lucide.icons)
    implementation(libs.androidx.navigation3.runtime)
    implementation(libs.androidx.navigation3.ui)

    testImplementation(libs.junit4)

    debugImplementation(libs.androidx.compose.ui.tooling)
}
