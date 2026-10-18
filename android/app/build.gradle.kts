import java.util.Properties

plugins {
    alias(libs.plugins.android.application)
    alias(libs.plugins.kotlin.android)
    alias(libs.plugins.kotlin.compose)
    alias(libs.plugins.kotlin.serialization)
}

// The version every surface shares, written once at the root of the repository.
val product = Properties().apply { rootProject.file("../version.properties").inputStream().use { load(it) } }

// Release signing is kept outside the repository: signing.properties holds storeFile (relative
// to that directory), storePassword, keyAlias and keyPassword. Without it a release build is
// signed with the debug key, which cannot update a published install.
val signingDir = File(System.getProperty("user.home"), ".config/keduly")
val signingFile = File(signingDir, "signing.properties")
val signing = Properties().apply { if (signingFile.exists()) signingFile.inputStream().use { load(it) } }
if (providers.gradleProperty("officialRelease").orNull == "true") {
    require(signingFile.exists()) { "an official release needs ${signingFile.path}" }
}

android {
    namespace = "io.github.lewin671.keduly"
    compileSdk = 36

    defaultConfig {
        applicationId = "io.github.lewin671.keduly"
        minSdk = 26
        targetSdk = 36
        versionCode = product.getProperty("androidVersionCode").toInt()
        versionName = product.getProperty("version")
    }

    signingConfigs {
        if (signingFile.exists()) {
            create("release") {
                storeFile = File(signingDir, signing.getProperty("storeFile"))
                storePassword = signing.getProperty("storePassword")
                keyAlias = signing.getProperty("keyAlias")
                keyPassword = signing.getProperty("keyPassword")
            }
        }
    }

    buildTypes {
        release {
            isMinifyEnabled = false
            signingConfig = signingConfigs.findByName("release") ?: signingConfigs.getByName("debug")
        }
    }

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }
    buildFeatures {
        compose = true
    }
    lint {
        abortOnError = true
        warningsAsErrors = false
    }
}

kotlin {
    jvmToolchain(17)
}

dependencies {
    implementation(platform(libs.compose.bom))
    implementation(libs.compose.ui)
    implementation(libs.compose.foundation)
    implementation(libs.compose.material3)
    implementation(libs.core.ktx)
    implementation(libs.activity.compose)
    implementation(libs.lifecycle.runtime.compose)
    implementation(libs.lifecycle.viewmodel.compose)
    implementation(libs.navigation.compose)
    implementation(libs.coroutines.android)
    implementation(libs.serialization.json)
    implementation(libs.okhttp)

    testImplementation(libs.junit)
    testImplementation(libs.coroutines.test)
}
