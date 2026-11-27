package com.gowind.uba;

/** 一次 flush 的结果。 */
public final class FlushResult {

    /** 本次 flush 是否成功（可重试失败为 false）。 */
    public final boolean success;
    /** 成功送达的事件数。 */
    public final int sent;
    /** 未送达的事件数（部分失败/丢弃）。 */
    public final int dropped;
    /** 失败是否值得补发（网络错误/5xx/429 为 true；4xx 确定性失败为 false）。 */
    public final boolean retryable;
    /** 失败原因。 */
    public final String message;

    FlushResult(boolean success, int sent, int dropped, boolean retryable, String message) {
        this.success = success;
        this.sent = sent;
        this.dropped = dropped;
        this.retryable = retryable;
        this.message = message;
    }

    @Override
    public String toString() {
        return "FlushResult{success=" + success + ", sent=" + sent + ", dropped=" + dropped
                + ", retryable=" + retryable + ", message=" + message + "}";
    }
}
