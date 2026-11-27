package com.gowind.uba

import java.io.IOException
import java.net.HttpURLConnection
import java.net.URL

/** HTTP 结果：statusCode 0 表示网络层失败（超时/连接拒绝），可重试。 */
class HttpResult(val statusCode: Int, val body: String) {
    val isOk: Boolean get() = statusCode in 200..299
}

/**
 * HTTP 传输抽象。默认实现用 HttpURLConnection（Android/JVM 通用，零依赖）；
 * 换 OkHttp / 走网关加签时实现本接口注入，无需改动 UbaClient。
 */
interface HttpTransport {
    fun send(url: String, body: String, timeoutMs: Int): HttpResult

    /** 释放底层资源（默认无连接池）。 */
    fun close() {}

    /** HttpURLConnection 默认实现。 */
    class UrlConnectionTransport : HttpTransport {
        override fun send(url: String, body: String, timeoutMs: Int): HttpResult {
            var conn: HttpURLConnection? = null
            return try {
                conn = (URL(url).openConnection() as HttpURLConnection).apply {
                    requestMethod = "POST"
                    connectTimeout = timeoutMs
                    readTimeout = timeoutMs
                    doOutput = true
                    setRequestProperty("Content-Type", "application/json")
                    setRequestProperty("Connection", "keep-alive")
                }
                conn.outputStream.use { it.write(body.toByteArray(Charsets.UTF_8)) }
                val status = conn.responseCode
                val stream = if (status in 200..299) conn.inputStream else conn.errorStream
                val text = stream?.bufferedReader(Charsets.UTF_8)?.use { it.readText() } ?: ""
                HttpResult(status, text)
            } catch (e: IOException) {
                HttpResult(0, e.message ?: "network error")
            } finally {
                conn?.disconnect()
            }
        }
    }
}
