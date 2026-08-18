package com.mediahub.android.playback

import android.content.Context
import androidx.media3.common.MediaItem
import androidx.media3.common.Player
import androidx.media3.datasource.DefaultDataSource
import androidx.media3.datasource.DefaultHttpDataSource
import androidx.media3.exoplayer.ExoPlayer
import androidx.media3.exoplayer.source.DefaultMediaSourceFactory
import androidx.test.core.app.ApplicationProvider
import androidx.test.ext.junit.runners.AndroidJUnit4
import androidx.test.platform.app.InstrumentationRegistry
import java.io.BufferedReader
import java.io.Closeable
import java.io.InputStreamReader
import java.net.InetAddress
import java.net.ServerSocket
import java.net.Socket
import java.util.Collections
import java.util.concurrent.CountDownLatch
import java.util.concurrent.TimeUnit
import kotlin.concurrent.thread
import org.junit.Assert.assertTrue
import org.junit.Test
import org.junit.runner.RunWith

@RunWith(AndroidJUnit4::class)
class Media3HttpTransportTest {
    @Test
    fun seekUsesRangeAndPreservesConfiguredUserAgent() {
        val context = ApplicationProvider.getApplicationContext<Context>()
        val media = context.assets.open("media3-fixture.mp4").use { it.readBytes() }
        ThrottledRangeServer(media).use { server ->
            val ready = CountDownLatch(1)
            lateinit var player: ExoPlayer
            InstrumentationRegistry.getInstrumentation().runOnMainSync {
                val http = DefaultHttpDataSource.Factory()
                    .setUserAgent(TEST_USER_AGENT)
                    .setAllowCrossProtocolRedirects(false)
                player = ExoPlayer.Builder(context)
                    .setMediaSourceFactory(DefaultMediaSourceFactory(DefaultDataSource.Factory(context, http)))
                    .build()
                player.addListener(object : Player.Listener {
                    override fun onPlaybackStateChanged(playbackState: Int) {
                        if (playbackState == Player.STATE_READY) ready.countDown()
                    }
                })
                player.setMediaItem(MediaItem.fromUri(server.url))
                player.prepare()
            }
            try {
                assertTrue("fixture never became ready", ready.await(15, TimeUnit.SECONDS))
                InstrumentationRegistry.getInstrumentation().runOnMainSync {
                    player.seekTo(20_000)
                }
                assertTrue("seek did not issue a positive Range request", server.positiveRange.await(15, TimeUnit.SECONDS))
                val requests = synchronized(server.requests) { server.requests.toList() }
                assertTrue(requests.isNotEmpty())
                assertTrue(requests.all { it.userAgent == TEST_USER_AGENT })
                assertTrue(requests.any { request ->
                    request.range?.removePrefix("bytes=")?.substringBefore('-')?.toLongOrNull()?.let { it > 0 } == true
                })
            } finally {
                InstrumentationRegistry.getInstrumentation().runOnMainSync { player.release() }
            }
        }
    }

    private companion object {
        const val TEST_USER_AGENT = "Media-Hub-Media3-Test/1"
    }
}

private data class RecordedRequest(val userAgent: String?, val range: String?)

private class ThrottledRangeServer(private val media: ByteArray) : Closeable {
    private val server = ServerSocket(0, 16, InetAddress.getByName("127.0.0.1"))
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

    private fun handle(socket: Socket) {
        socket.use { client ->
            runCatching {
                client.soTimeout = 5_000
                val reader = BufferedReader(InputStreamReader(client.getInputStream(), Charsets.US_ASCII))
                val requestLine = reader.readLine().orEmpty()
                val headers = linkedMapOf<String, String>()
                while (true) {
                    val line = reader.readLine() ?: break
                    if (line.isEmpty()) break
                    val separator = line.indexOf(':')
                    if (separator > 0) headers[line.substring(0, separator).trim().lowercase()] = line.substring(separator + 1).trim()
                }
                val range = headers["range"]
                val start = range?.removePrefix("bytes=")?.substringBefore('-')?.toLongOrNull()?.coerceIn(0, media.lastIndex.toLong()) ?: 0L
                val end = range?.substringAfter('-', "")?.toLongOrNull()?.coerceIn(start, media.lastIndex.toLong()) ?: media.lastIndex.toLong()
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
            }
        }
    }

    override fun close() {
        server.close()
        worker.join(1_000)
    }
}
