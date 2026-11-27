// session.ets —— 会话管理：后台超时轮换 + 事件间隔超时轮换（对齐 Go/Java/Kotlin 语义）。
import { nowMs, uuidV4 } from './utils'

export class SessionManager {
  private id: string = uuidV4()
  private pausedAt: number | null = null
  private lastEventAt: number | null = null
  private readonly timeoutMs: number
  private readonly nowProvider: () => number

  constructor(timeoutMs: number = 30000, nowProvider: (() => number) | null = null) {
    this.timeoutMs = timeoutMs
    this.nowProvider = nowProvider !== null ? nowProvider : nowMs
  }

  get current(): string {
    return this.id
  }

  inSample(sampleRate: number): boolean {
    // 与各端 SDK 一致的确定性采样
    if (sampleRate >= 1) {
      return true
    }
    if (sampleRate <= 0) {
      return false
    }
    let hash = 0x811c9dc5
    for (let i = 0; i < this.id.length; i++) {
      hash = hash ^ (this.id.charCodeAt(i) & 0xff)
      hash = Math.imul(hash, 0x01000193) >>> 0
    }
    return (hash >>> 0) % 1000 / 1000 < sampleRate
  }

  /** 进入后台（onPause/onStop）。 */
  onBackground(): void {
    if (this.pausedAt === null) {
      this.pausedAt = this.nowProvider()
    }
  }

  /** 回到前台：后台超过 timeoutMs 则轮换新会话。 */
  onForeground(): void {
    const paused = this.pausedAt
    this.pausedAt = null
    if (paused !== null && this.timeoutMs > 0 && this.nowProvider() - paused > this.timeoutMs) {
      this.rotate()
    }
  }

  /** 事件间隔超时轮换（服务端/常驻场景语义）。返回当前（可能已轮换的）会话 ID。 */
  checkTimeout(): string {
    const now = this.nowProvider()
    if (this.lastEventAt !== null && this.timeoutMs > 0 &&
      now - this.lastEventAt > this.timeoutMs) {
      this.rotate()
    }
    this.lastEventAt = now
    return this.id
  }

  /** 手动开启新会话。 */
  rotate(): void {
    this.id = uuidV4()
    this.pausedAt = null
  }
}
