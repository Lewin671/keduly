// Letting another device in from this phone: the approval link it shared, then the same
// approval as the web app. The pin is only ever typed here, read off the other screen.
package io.github.lewin671.keduly.ui

import android.content.ClipboardManager
import android.content.Context
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import io.github.lewin671.keduly.R
import io.github.lewin671.keduly.api.ApiException
import io.github.lewin671.keduly.api.PendingLogin
import io.github.lewin671.keduly.core.ApprovalLink
import io.github.lewin671.keduly.core.parseApprovalLink
import kotlinx.coroutines.launch

private sealed interface Approval {
    data object Paste : Approval
    data object Loading : Approval
    data class Asking(val request: PendingLogin) : Approval
    data object Done : Approval
    data object Gone : Approval
}

@Composable
fun ApproveScreen() {
    val app = LocalApp.current
    val nav = LocalNav.current
    val context = LocalContext.current
    val scope = rememberCoroutineScope()
    val boot by app.boot.collectAsStateWithLifecycle()
    var step by remember { mutableStateOf<Approval>(Approval.Paste) }
    var link by remember { mutableStateOf("") }
    var pin by remember { mutableStateOf("") }
    var error by remember { mutableStateOf<String?>(null) }
    var busy by remember { mutableStateOf(false) }

    // A request that is unknown, expired, used, refused or out of pins is a 404, whichever call met it.
    fun failed(e: ApiException) {
        if (e.status == 404) step = Approval.Gone else error = app.describe(e)
    }

    fun read(text: String) {
        link = text
        error = null
        when (val parsed = parseApprovalLink(text, app.store.server.orEmpty())) {
            ApprovalLink.NotALink -> if (text.isNotBlank()) error = app.text(R.string.approve_not_link)
            is ApprovalLink.OtherServer -> error = app.text(R.string.approve_other_server, parsed.host)
            is ApprovalLink.Request -> {
                step = Approval.Loading
                scope.launch {
                    try {
                        step = Approval.Asking(app.api.loginRequest(parsed.id))
                    } catch (e: ApiException) {
                        step = Approval.Paste
                        failed(e)
                    }
                }
            }
        }
    }

    Screen(stringResource(R.string.approve_title), grouped = true) {
        Scroll {
            when (val s = step) {
                Approval.Paste, Approval.Loading -> {
                    Text(stringResource(R.string.approve_paste_lead), fontSize = 15.sp, lineHeight = 21.sp, color = K.label2, modifier = Modifier.padding(vertical = 12.dp))
                    OutlinedTextField(
                        link, ::read, Modifier.fillMaxWidth().testTag("approve-link"), singleLine = true, colors = fieldColors(),
                        placeholder = { Text(stringResource(R.string.approve_link)) }, enabled = s == Approval.Paste,
                        trailingIcon = {
                            TextButton(enabled = s == Approval.Paste, onClick = {
                                val clip = (context.getSystemService(Context.CLIPBOARD_SERVICE) as ClipboardManager).primaryClip
                                read(clip?.takeIf { it.itemCount > 0 }?.getItemAt(0)?.coerceToText(context)?.toString().orEmpty())
                            }) { Text(stringResource(R.string.approve_paste), color = K.tint) }
                        },
                    )
                    FormError(error)
                    if (s == Approval.Loading) CircularProgressIndicator(Modifier.padding(top = 24.dp).align(Alignment.CenterHorizontally), color = K.tint)
                }
                is Approval.Asking -> Column(Modifier.fillMaxWidth().padding(top = 24.dp), horizontalAlignment = Alignment.CenterHorizontally) {
                    val email = boot?.user?.email.orEmpty()
                    Text(stringResource(R.string.approve_ask), fontSize = 22.sp, fontWeight = FontWeight.Bold)
                    Text(
                        if (s.request.device.isEmpty()) stringResource(R.string.approve_lead_unknown, email) else stringResource(R.string.approve_lead, s.request.device, email),
                        fontSize = 15.sp, lineHeight = 21.sp, color = K.label2, textAlign = TextAlign.Center, modifier = Modifier.padding(top = 10.dp),
                    )
                    OutlinedTextField(
                        pin, { value -> pin = value.filter(Char::isDigit).take(4); error = null },
                        Modifier.padding(top = 22.dp).width(200.dp).testTag("approve-pin"), singleLine = true, colors = fieldColors(),
                        label = { Text(stringResource(R.string.approve_pin), fontSize = 13.sp) },
                        textStyle = LocalTextStyle.current.copy(fontSize = 28.sp, letterSpacing = 10.sp, textAlign = TextAlign.Center),
                        keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.NumberPassword),
                    )
                    FormError(error)
                    Text(stringResource(R.string.approve_warning), fontSize = 13.sp, lineHeight = 18.sp, color = K.orange, textAlign = TextAlign.Center, modifier = Modifier.padding(vertical = 18.dp))
                    PillButton(stringResource(R.string.approve_allow), Modifier.fillMaxWidth().testTag("approve-allow"), primary = true, enabled = pin.length == 4 && !busy) {
                        busy = true
                        scope.launch {
                            try {
                                app.api.approveLoginRequest(s.request.id, pin)
                                step = Approval.Done
                            } catch (e: ApiException) {
                                // A wrong pin: the field clears for another look at the other screen.
                                if (e.status == 400) {
                                    pin = ""
                                    error = app.text(R.string.approve_wrong_pin)
                                } else failed(e)
                            } finally {
                                busy = false
                            }
                        }
                    }
                    TextButton(enabled = !busy, onClick = {
                        busy = true
                        scope.launch {
                            try {
                                app.api.refuseLoginRequest(s.request.id)
                                nav.back()
                            } catch (e: ApiException) {
                                if (e.status == 404) nav.back() else error = app.describe(e)
                            } finally {
                                busy = false
                            }
                        }
                    }, modifier = Modifier.padding(top = 8.dp).testTag("approve-refuse")) { Text(stringResource(R.string.approve_refuse), color = K.tint, fontSize = 15.sp) }
                }
                Approval.Done, Approval.Gone -> Column(Modifier.fillMaxWidth().padding(top = 24.dp), horizontalAlignment = Alignment.CenterHorizontally) {
                    val done = s == Approval.Done
                    Text(stringResource(if (done) R.string.approve_done else R.string.approve_gone), fontSize = 22.sp, fontWeight = FontWeight.Bold)
                    Text(
                        stringResource(if (done) R.string.approve_done_lead else R.string.approve_gone_lead),
                        fontSize = 15.sp, lineHeight = 21.sp, color = K.label2, textAlign = TextAlign.Center, modifier = Modifier.padding(vertical = 12.dp),
                    )
                    PillButton(stringResource(R.string.ok), Modifier.fillMaxWidth().padding(top = 8.dp), primary = true, onClick = nav.back)
                }
            }
        }
    }
}
