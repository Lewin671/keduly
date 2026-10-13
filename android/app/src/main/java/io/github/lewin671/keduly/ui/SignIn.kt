// The first screen: the server address, the email and the password. Registering is offered when
// the server says it is open.
package io.github.lewin671.keduly.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.KeyboardActions
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import io.github.lewin671.keduly.R
import io.github.lewin671.keduly.api.ApiException
import io.github.lewin671.keduly.api.normalizeServer
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch

@Composable
fun fieldColors(): TextFieldColors = OutlinedTextFieldDefaults.colors(
    focusedBorderColor = K.tint, unfocusedBorderColor = K.label3, focusedLabelColor = K.tint, unfocusedLabelColor = K.label2,
    cursorColor = K.tint, focusedTextColor = K.label, unfocusedTextColor = K.label,
)

@Composable
fun SignIn() {
    val app = LocalApp.current
    val scope = rememberCoroutineScope()
    val busy by app.busy.collectAsStateWithLifecycle()
    var server by rememberSaveable { mutableStateOf(app.store.server.orEmpty()) }
    var email by rememberSaveable { mutableStateOf("") }
    var password by rememberSaveable { mutableStateOf("") }
    var name by rememberSaveable { mutableStateOf("") }
    var registering by rememberSaveable { mutableStateOf(false) }
    var open by remember { mutableStateOf(false) }
    var error by remember { mutableStateOf<String?>(null) }
    var authorizationServer by remember { mutableStateOf<String?>(null) }

    // Ask the server whether it takes new accounts once the address has settled.
    LaunchedEffect(server) {
        open = false
        val origin = normalizeServer(server) ?: return@LaunchedEffect
        delay(500)
        app.store.server = origin
        open = try {
            app.api.config().registration == "open"
        } catch (e: ApiException) {
            false
        }
        if (!open) registering = false
    }

    authorizationServer?.let { origin -> DeviceAuthorization(origin) { authorizationServer = null } }

    val ready = normalizeServer(server) != null && email.isNotBlank() && password.isNotEmpty() && (!registering || (name.isNotBlank() && password.length >= 8))
    val submit = {
        val origin = normalizeServer(server)
        if (origin != null && ready && !busy) scope.launch { error = app.signIn(origin, email.trim(), password, if (registering) name.trim() else null) }
        Unit
    }

    Column(
        Modifier.fillMaxSize().background(K.bg).systemBarsPadding().imePadding().verticalScroll(rememberScrollState()).padding(horizontal = 28.dp),
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        Spacer(Modifier.height(64.dp))
        Logo(64.dp)
        Spacer(Modifier.height(14.dp))
        Text("Keduly", fontSize = 28.sp, fontWeight = FontWeight.Bold, color = K.label)
        Text(stringResource(if (registering) R.string.auth_register_lead else R.string.auth_login_lead), fontSize = 15.sp, color = K.label2, modifier = Modifier.padding(top = 4.dp))
        Spacer(Modifier.height(28.dp))
        OutlinedTextField(
            server, { server = it; error = null }, Modifier.fillMaxWidth(), singleLine = true, colors = fieldColors(),
            label = { Text(stringResource(R.string.auth_server)) }, placeholder = { Text("keduly.example.com", color = K.label3) },
            keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Uri, imeAction = ImeAction.Next, autoCorrectEnabled = false),
        )
        Spacer(Modifier.height(10.dp))
        if (registering) {
            OutlinedTextField(
                name, { name = it; error = null }, Modifier.fillMaxWidth(), singleLine = true, colors = fieldColors(),
                label = { Text(stringResource(R.string.auth_name)) }, keyboardOptions = KeyboardOptions(imeAction = ImeAction.Next),
            )
            Spacer(Modifier.height(10.dp))
        }
        OutlinedTextField(
            email, { email = it; error = null }, Modifier.fillMaxWidth(), singleLine = true, colors = fieldColors(),
            label = { Text(stringResource(R.string.auth_email)) },
            keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Email, imeAction = ImeAction.Next, autoCorrectEnabled = false),
        )
        Spacer(Modifier.height(10.dp))
        OutlinedTextField(
            password, { password = it; error = null }, Modifier.fillMaxWidth(), singleLine = true, colors = fieldColors(),
            label = { Text(stringResource(R.string.auth_password)) }, visualTransformation = PasswordVisualTransformation(),
            supportingText = if (registering) ({ Text(stringResource(R.string.auth_password_tip)) }) else null,
            keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Password, imeAction = ImeAction.Done),
            keyboardActions = KeyboardActions(onDone = { submit() }),
        )
        error?.let { Text(it, fontSize = 14.sp, color = K.red, modifier = Modifier.fillMaxWidth().padding(top = 10.dp)) }
        Spacer(Modifier.height(18.dp))
        PillButton(stringResource(if (registering) R.string.auth_register else R.string.auth_login), Modifier.fillMaxWidth(), primary = true, enabled = ready && !busy, onClick = submit)
        if (!registering) {
            TextButton(onClick = { authorizationServer = normalizeServer(server) }, enabled = normalizeServer(server) != null && !busy) {
                Text(stringResource(R.string.auth_authorize), color = K.tint, fontSize = 15.sp)
            }
        }
        if (open) {
            TextButton(onClick = { registering = !registering; error = null }, modifier = Modifier.padding(top = 6.dp)) {
                Text(stringResource(if (registering) R.string.auth_to_login else R.string.auth_to_register), color = K.tint, fontSize = 15.sp)
            }
        }
        Spacer(Modifier.height(32.dp))
    }
}

/** The app icon: a white K on the brand blue. */
@Composable
fun Logo(size: androidx.compose.ui.unit.Dp) {
    androidx.compose.foundation.Canvas(Modifier.size(size).clip(RoundedCornerShape(size * 7.5f / 32f)).background(androidx.compose.ui.graphics.Color(0xFF2867D6))) {
        val u = this.size.width / 32f
        val white = androidx.compose.ui.graphics.Color.White
        val cap = androidx.compose.ui.graphics.StrokeCap.Round
        fun at(x: Float, y: Float) = androidx.compose.ui.geometry.Offset(x * u, y * u)
        drawLine(white, at(9.5f, 8.5f), at(9.5f, 23.5f), 3f * u, cap)
        drawLine(white, at(22f, 8.5f), at(12f, 16f), 3f * u, cap)
        drawLine(white, at(12f, 16f), at(22f, 23.5f), 3f * u, cap)
    }
}
