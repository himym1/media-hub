package com.mediahub.android.playback

import java.io.BufferedReader
import java.io.Closeable
import java.io.InputStreamReader
import java.net.InetAddress
import java.net.ServerSocket
import java.net.Socket
import java.net.SocketException
import java.util.Collections
import java.util.concurrent.CountDownLatch
import java.util.concurrent.atomic.AtomicReference
import kotlin.concurrent.thread

internal data class RecordedRequest(val userAgent: String?, val range: String?)

internal class ThrottledRangeServer(private val media: ByteArray) : Closeable {
    private val server = ServerSocket(0, 16, InetAddress.getByName("127.0.0.1"))
    private val firstFailure = AtomicReference<Throwable?>(null)
    private val worker = thread(name = "media3-range-server", isDaemon = true) {
        while (!server.isClosed) {
            val socket = try {
                server.accept()
            } catch (_: Exception) {
                break
            }
            handle(socket)
        }
    }
    val requests = Collections.synchronizedList(mutableListOf<RecordedRequest>())
    val positiveRange = CountDownLatch(1)
    val url: String = "http://127.0.0.1:${server.localPort}/media3-fixture.mp4"

    fun assertNoUnexpectedFailures() {
        firstFailure.get()?.let { throw AssertionError("local Range server failed", it) }
    }

    private fun handle(socket: Socket) {
        socket.use { client ->
            try {
                client.soTimeout = 5_000
                val reader = BufferedReader(InputStreamReader(client.getInputStream(), Charsets.US_ASCII))
                val requestLine = reader.readLine().orEmpty()
                val headers = linkedMapOf<String, String>()
                while (true) {
                    val line = reader.readLine() ?: break
                    if (line.isEmpty()) break
                    val separator = line.indexOf(':')
                    if (separator > 0) {
                        headers[line.substring(0, separator).trim().lowercase()] = line.substring(separator + 1).trim()
                    }
                }
                val range = headers["range"]
                val start = range?.removePrefix("bytes=")?.substringBefore('-')?.toLongOrNull()
                    ?.coerceIn(0, media.lastIndex.toLong()) ?: 0L
                val end = range?.substringAfter('-', "")?.toLongOrNull()
                    ?.coerceIn(start, media.lastIndex.toLong()) ?: media.lastIndex.toLong()
                requests += RecordedRequest(headers["user-agent"], range)
                if (range != null && start > 0) positiveRange.countDown()
                val partial = range != null
                val length = end - start + 1
                val output = client.getOutputStream()
                output.write((if (partial) "HTTP/1.1 206 Partial Content\r\n" else "HTTP/1.1 200 OK\r\n").toByteArray())
                output.write("Content-Type: video/mp4\r\n".toByteArray())
                output.write("Accept-Ranges: bytes\r\n".toByteArray())
                output.write("Content-Length: $length\r\n".toByteArray())
                if (partial) output.write("Content-Range: bytes $start-$end/${media.size}\r\n".toByteArray())
                output.write("Connection: close\r\n\r\n".toByteArray())
                if (!requestLine.startsWith("HEAD ")) {
                    var offset = start.toInt()
                    val last = end.toInt()
                    while (offset <= last) {
                        val count = minOf(4 * 1024, last - offset + 1)
                        output.write(media, offset, count)
                        output.flush()
                        offset += count
                        Thread.sleep(30)
                    }
                }
            } catch (error: Throwable) {
                if (!expectedClientDisconnect(error)) firstFailure.compareAndSet(null, error)
            }
        }
    }

    private fun expectedClientDisconnect(error: Throwable): Boolean {
        if (error !is SocketException) return false
        val message = error.message.orEmpty().lowercase()
        return "broken pipe" in message || "connection reset" in message || "socket closed" in message
    }

    override fun close() {
        server.close()
        worker.join(1_000)
        assertNoUnexpectedFailures()
    }
}
