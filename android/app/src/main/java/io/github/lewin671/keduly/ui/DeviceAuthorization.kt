package io.github.lewin671.keduly.ui

import android.content.Intent
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.text.selection.SelectionContainer
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.LocalUriHandler
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.compose.ui.window.Dialog
import androidx.lifecycle.Lifecycle
import androidx.lifecycle.compose.LocalLifecycleOwner
import io.github.lewin671.keduly.R
import io.github.lewin671.keduly.api.ApiException
import io.github.lewin671.keduly.api.DeviceLoginRequest
import java.time.Instant
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.delay
import kotlinx.coroutines.ensureActive
import kotlin.coroutines.coroutineContext

/** Only the public request id goes into the shared link. The claim secret remains in memory. */
@Composable
fun DeviceAuthorization(server: String, close: () -> Unit) {
    val app = LocalApp.current
    val context = LocalContext.current
    val uriHandler = LocalUriHandler.current
    val lifecycle = LocalLifecycleOwner.current.lifecycle
    var request by remember { mutableStateOf<DeviceLoginRequest?>(null) }
    var error by remember { mutableStateOf<String?>(null) }
    var attempt by remember { mutableIntStateOf(0) }

    LaunchedEffect(attempt) {
        request = null
        error = null
        try {
            app.store.server = server
            val current = app.api.startLoginRequest()
            coroutineContext.ensureActive()
            request = current
            val expires = Instant.parse(current.request.expiresAt)
            while (Instant.now().isBefore(expires)) {
                delay(2000)
                if (!lifecycle.currentState.isAtLeast(Lifecycle.State.RESUMED)) continue
                if (!Instant.now().isBefore(expires)) break
                if (app.api.claimLoginRequest(current)) {
                    coroutineContext.ensureActive()
                    app.finishDeviceSignIn()
                    close()
                    return@LaunchedEffect
                }
            }
            error = app.text(R.string.auth_authorization_expired)
        } catch (e: CancellationException) {
            throw e
        } catch (e: ApiException) {
            error = if (e.status == 404) app.text(R.string.auth_authorization_expired) else app.describe(e)
        } catch (e: Exception) {
            error = app.text(R.string.error_bad_response)
        }
    }

    Dialog(onDismissRequest = close) {
        Surface(shape = MaterialTheme.shapes.extraLarge, color = K.cell) {
            Column(Modifier.padding(24.dp).verticalScroll(rememberScrollState()), verticalArrangement = Arrangement.spacedBy(16.dp)) {
                Text(stringResource(R.string.auth_authorization_lead), fontSize = 22.sp, fontWeight = FontWeight.Bold, color = K.label)
                Text(stringResource(R.string.auth_authorization_help), color = K.label2)
                val current = request
                if (current != null) {
                    Text(current.pin, fontSize = 36.sp, fontWeight = FontWeight.SemiBold, color = K.label)
                    if (error == null) {
                        val link = current.approvalUrl(server)
                        Text(stringResource(R.string.auth_authorization_waiting), color = K.label2)
                        TextButton(onClick = { uriHandler.openUri(link) }) { Text(stringResource(R.string.auth_authorization_open)) }
                        TextButton(onClick = {
                            val intent = Intent(Intent.ACTION_SEND).setType("text/plain").putExtra(Intent.EXTRA_TEXT, link)
                            context.startActivity(Intent.createChooser(intent, null))
                        }) { Text(stringResource(R.string.auth_authorization_share)) }
                        SelectionContainer { Text(link, fontSize = 12.sp, color = K.label2) }
                    }
                } else if (error == null) {
                    CircularProgressIndicator()
                }
                error?.let {
                    Text(it, color = K.red)
                    TextButton(onClick = { attempt++ }) { Text(stringResource(R.string.auth_authorization_retry)) }
                }
                TextButton(onClick = close) { Text(stringResource(R.string.auth_authorization_cancel)) }
            }
        }
    }
}
