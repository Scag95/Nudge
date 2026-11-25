package com.clipboardshare

import java.net.InetAddress
import java.net.Socket
import javax.net.SocketFactory

/**
 * Wraps [delegate] and turns Nagle's algorithm off (TCP_NODELAY) on every socket it
 * creates, so small `move` frames leave immediately instead of waiting to be batched.
 * Only the raw TCP socket is touched: OkHttp layers TLS (and the certificate pin) on top
 * of whatever this factory returns, exactly as with the default factory.
 */
class NoDelaySocketFactory(private val delegate: SocketFactory) : SocketFactory() {
    private fun Socket.noDelay(): Socket = apply { tcpNoDelay = true }

    override fun createSocket(): Socket = delegate.createSocket().noDelay()

    override fun createSocket(host: String, port: Int): Socket =
        delegate.createSocket(host, port).noDelay()

    override fun createSocket(host: String, port: Int, localHost: InetAddress, localPort: Int): Socket =
        delegate.createSocket(host, port, localHost, localPort).noDelay()

    override fun createSocket(host: InetAddress, port: Int): Socket =
        delegate.createSocket(host, port).noDelay()

    override fun createSocket(address: InetAddress, port: Int, localAddress: InetAddress, localPort: Int): Socket =
        delegate.createSocket(address, port, localAddress, localPort).noDelay()
}
