package io.github.lewin671.keduly

import io.github.lewin671.keduly.core.ApprovalLink
import io.github.lewin671.keduly.core.parseApprovalLink
import org.junit.Assert.assertEquals
import org.junit.Test

class ApprovalLinkTest {
    private val server = "https://keduly.example.com"
    private val id = "cuzdsntqa4txq2ct"

    @Test fun takesTheIdOfALinkForThisServer() {
        assertEquals(ApprovalLink.Request(id), parseApprovalLink("https://keduly.example.com/#approve=$id", server))
        assertEquals(ApprovalLink.Request(id), parseApprovalLink("  请打开 https://Keduly.Example.com:443/#approve=$id 批准\n", server))
        assertEquals(ApprovalLink.Request(id), parseApprovalLink(id, server))
    }

    @Test fun refusesAnotherServer() {
        assertEquals(ApprovalLink.OtherServer("keduly.example.org"), parseApprovalLink("https://keduly.example.org/#approve=$id", server))
        assertEquals(ApprovalLink.OtherServer("keduly.example.com"), parseApprovalLink("http://keduly.example.com/#approve=$id", server))
        assertEquals(ApprovalLink.OtherServer("keduly.example.com"), parseApprovalLink("https://keduly.example.com:8443/#approve=$id", server))
    }

    @Test fun refusesAnythingElse() {
        for (text in listOf("", "hello", "https://keduly.example.com/", "https://keduly.example.com/#approve=short", "https://keduly.example.com/#approve=UPPERCASEUPPERCAS"))
            assertEquals(text, ApprovalLink.NotALink, parseApprovalLink(text, server))
    }
}
