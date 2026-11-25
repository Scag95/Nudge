package com.clipboardshare

import org.junit.Assert.assertTrue
import org.junit.Test
import java.net.InetAddress
import java.net.ServerSocket
import javax.net.SocketFactory

class NoDelaySocketFactoryTest {
    private val factory = NoDelaySocketFactory(SocketFactory.getDefault())

    @Test fun unconnectedSocketsHaveNagleOff() {
        factory.createSocket().use { assertTrue(it.tcpNoDelay) }
    }

    @Test fun connectedSocketsHaveNagleOff() {
        ServerSocket(0, 50, InetAddress.getLoopbackAddress()).use { server ->
            factory.createSocket(InetAddress.getLoopbackAddress(), server.localPort).use {
                assertTrue(it.tcpNoDelay)
            }
            factory.createSocket("127.0.0.1", server.localPort).use { assertTrue(it.tcpNoDelay) }
        }
    }

    @Test fun theHttpClientsUseIt() {
        assertTrue(InputClient.defaultHttpClient().socketFactory is NoDelaySocketFactory)
    }

    @Test fun pinningKeepsTheSocketFactory() {
        val pinned = PinnedTls.client(InputClient.defaultHttpClient(), "AB:CD")
        assertTrue(pinned.http.socketFactory is NoDelaySocketFactory)
    }
}
