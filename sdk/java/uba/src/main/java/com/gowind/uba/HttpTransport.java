package com.gowind.uba;

import java.io.IOException;
import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.time.Duration;

/** HTTP 传输抽象：默认 JDK HttpClient；换网关/加签/连接池时实现本接口注入。 */
public interface HttpTransport {

    /** statusCode 0 表示网络层失败（超时/连接拒绝），可重试。 */
    HttpResult postJson(String url, String bodyJson, Duration timeout) throws InterruptedException;

    /** 关闭底层资源。 */
    default void close() {}

    /** 传输结果。 */
    final class HttpResult {
        public final int statusCode;
        public final String body;

        public HttpResult(int statusCode, String body) {
            this.statusCode = statusCode;
            this.body = body;
        }

        public boolean isOk() {
            return statusCode >= 200 && statusCode < 300;
        }
    }

    /** JDK 11+ 内置 HttpClient 实现（HTTP/2、连接复用）。 */
    class JdkHttpTransport implements HttpTransport {

        private final HttpClient client = HttpClient.newBuilder()
                .version(HttpClient.Version.HTTP_1_1)
                .connectTimeout(Duration.ofSeconds(3))
                .build();

        @Override
        public HttpResult postJson(String url, String bodyJson, Duration timeout)
                throws InterruptedException {
            HttpRequest request = HttpRequest.newBuilder()
                    .uri(URI.create(url))
                    .timeout(timeout)
                    .header("Content-Type", "application/json")
                    .POST(HttpRequest.BodyPublishers.ofString(bodyJson))
                    .build();
            try {
                HttpResponse<String> response =
                        client.send(request, HttpResponse.BodyHandlers.ofString());
                return new HttpResult(response.statusCode(), response.body());
            } catch (IOException e) {
                // 网络失败统一转 statusCode 0
                return new HttpResult(0, String.valueOf(e.getMessage()));
            }
        }
    }
}
