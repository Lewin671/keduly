package io.github.lewin671.keduly

import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.jsonObject
import io.github.lewin671.keduly.api.ApiJson

/** JSON fixtures written from docs/api.md, under `src/test/resources/fixtures`. */
object Fixtures {
    fun text(name: String): String =
        checkNotNull(javaClass.getResourceAsStream("/fixtures/$name.json")) { "no fixture $name" }.bufferedReader().use { it.readText() }

    fun json(name: String): JsonObject = ApiJson.parseToJsonElement(text(name)).jsonObject
}
