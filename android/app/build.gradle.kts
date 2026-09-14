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

val releaseVersionCode = providers.gradleProperty("MEDIA_HUB_VERSION_CODE").orElse("20048").get().toInt()
val releaseVersionName = providers.gradleProperty("MEDIA_HUB_VERSION_NAME").orElse("0.20.48").get()

val mediaHubApiBaseUrl = providers.gradleProperty("MEDIA_HUB_API_BASE_URL")
    .orElse("https://media.himym.us.ci")
    .get()
    .replace("\\", "\\\\")
    .replace("\"", "\\\"")

android {
    namespace = "com.mediahub.android"
    compileSdk = 37
    buildToolsVersion = "37.0.0"

    defaultConfig {
        applicationId = "com.mediahub.android"
        minSdk = 26
        targetSdk = 37
        versionCode = releaseVersionCode
        versionName = releaseVersionName
        buildConfigField("String", "API_BASE_URL", "\"$mediaHubApiBaseUrl\"")
        testInstrumentationRunner = "androidx.test.runner.AndroidJUnitRunner"
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
            applicationIdSuffix = ".debug"
            versionNameSuffix = "-debug"
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
    implementation(libs.androidx.compose.material3)
    implementation(libs.androidx.compose.material3.adaptive)
    implementation(libs.androidx.compose.material3.adaptive.layout)
    implementation(libs.kotlinx.coroutines.android)
    implementation(libs.lucide.icons)
    implementation(libs.androidx.navigation3.runtime)
    implementation(libs.androidx.navigation3.ui)
    implementation(libs.androidx.media3.exoplayer)
    implementation(libs.androidx.media3.ui)
    implementation(libs.androidx.media3.session)
    implementation(libs.nextlib.media3ext)

    testImplementation(libs.junit4)
    testImplementation(libs.json)
    androidTestImplementation(libs.androidx.test.ext.junit)
    androidTestImplementation(libs.androidx.test.runner)
    androidTestImplementation(libs.androidx.compose.ui.test.junit4)

    debugImplementation(libs.androidx.compose.ui.tooling)
    debugImplementation(libs.androidx.compose.ui.test.manifest)
}
