plugins {
    id("com.android.application")
    id("org.jetbrains.kotlin.android")
}

// Passed by `just apk`: -PversionName, -PversionCode, -PserverUrl.
fun property(name: String, default: String) = providers.gradleProperty(name).getOrElse(default)

android {
    namespace = "app.fdroidselfhost"
    compileSdk = 34

    defaultConfig {
        applicationId = "app.fdroidselfhost"
        minSdk = 24
        targetSdk = 34
        versionCode = property("versionCode", "1").toInt()
        versionName = property("versionName", "0.0.0")
        // Pre-fills the server URL field; users can always change it in the app.
        buildConfigField("String", "DEFAULT_SERVER_URL", "\"${property("serverUrl", "")}\"")
    }

    buildFeatures {
        buildConfig = true
    }

    // Releases are signed with the key from the environment (see docs/android.md).
    // Without one, fall back to the debug key so local and CI builds still work.
    signingConfigs {
        System.getenv("APP_KEYSTORE")?.let { keystore ->
            create("release") {
                storeFile = file(keystore)
                storePassword = System.getenv("APP_KS_PASS")
                keyAlias = System.getenv("APP_KEY_ALIAS")
                keyPassword = System.getenv("APP_KEY_PASS")
            }
        }
    }

    buildTypes {
        release {
            signingConfig = signingConfigs.findByName("release") ?: signingConfigs.getByName("debug")
        }
    }

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }
    kotlinOptions {
        jvmTarget = "17"
    }
}
