import { uuidV4, sessionIdInSample } from './utils';

/** 会话管理：进程内 uuid，后台超过 timeout 后回到前台时轮换新会话。 */
export class SessionManager {
  private id: string = uuidV4();
  private pausedAt: number | null = null;

  constructor(
    private readonly timeout: number = 30000,
    private readonly now: () => number = () => Date.now(),
  ) {}

  current(): string {
    return this.id;
  }

  inSample(sampleRate: number): boolean {
    return sessionIdInSample(this.id, sampleRate);
  }

  onBackground(): void {
    if (this.pausedAt === null) this.pausedAt = this.now();
  }

  onForeground(): void {
    const paused = this.pausedAt;
    this.pausedAt = null;
    if (paused !== null && this.timeout > 0 && this.now() - paused > this.timeout) {
      this.rotate();
    }
  }

  rotate(): void {
    this.id = uuidV4();
    this.pausedAt = null;
  }
}
